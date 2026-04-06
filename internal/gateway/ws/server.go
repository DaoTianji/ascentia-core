package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"ascentia-core/internal/runtime"
)

// Config tunes the WebSocket gateway (auth, origin, limits, rate).
type Config struct {
	CheckOrigin func(*http.Request) bool

	AuthMode      AuthMode
	BearerToken   string
	JWTSigningKey []byte
	// AllowQueryBearer: when true, also accept ?bearer_token= matching WS_BEARER_TOKEN (browser WS cannot set headers).
	AllowQueryBearer bool
	// AllowQueryJWT: when true, also accept ?access_token= JWT (use only with HTTPS; URLs may be logged).
	AllowQueryJWT bool

	ReadLimit        int64
	MaxMsgsPerMinute int // 0 = disabled
}

type Server struct {
	Chat      *runtime.Service
	Addr      string
	Path      string
	Upgrader  websocket.Upgrader
	ReadLimit int64

	authMode         AuthMode
	bearerToken      string
	jwtKey           []byte
	allowQueryBearer bool
	allowQueryJWT    bool
	msgRate          *messageRateLimiter
}

func NewServer(addr string, path string, chat *runtime.Service, cfg Config) *Server {
	readLimit := cfg.ReadLimit
	if readLimit <= 0 {
		readLimit = 1 << 20
	}
	checkOrigin := cfg.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = func(r *http.Request) bool { return true }
	}
	return &Server{
		Chat: chat,
		Addr: addr,
		Path: path,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     checkOrigin,
		},
		ReadLimit:        readLimit,
		authMode:         cfg.AuthMode,
		bearerToken:      strings.TrimSpace(cfg.BearerToken),
		jwtKey:           cfg.JWTSigningKey,
		allowQueryBearer: cfg.AllowQueryBearer,
		allowQueryJWT:    cfg.AllowQueryJWT,
		msgRate:          newMessageRateLimiter(cfg.MaxMsgsPerMinute),
	}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.Path, s.handleWS)

	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("[ws] listening on %s%s", s.Addr, s.Path)
	return srv.ListenAndServe()
}

type notifier struct {
	w *wsConnWriter
}

func (n *notifier) SendSystemStatus(ctx context.Context, sessionID string, text string) error {
	msg := SystemStatusMessage{
		Type:      "system_status",
		SessionID: sessionID,
		Text:      text,
	}
	return n.w.writeJSON(msg)
}

func (n *notifier) SendAssistantStream(ctx context.Context, sessionID string, text string, isFinal bool) error {
	msg := AssistantStreamMessage{
		Type:      "assistant_stream",
		SessionID: sessionID,
		Text:      text,
		IsFinal:   isFinal,
	}
	return n.w.writeJSON(msg)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	var jwtUserID string

	switch s.authMode {
	case AuthBearer:
		if err := authBearer(r, s.bearerToken, s.allowQueryBearer); err != nil {
			log.Printf("[ws] auth bearer: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	case AuthJWT:
		res, err := authJWTRequest(r, s.jwtKey, s.allowQueryJWT)
		if err != nil {
			log.Printf("[ws] auth jwt: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		jwtUserID = res.JWTUserID
	}

	connCtx := runtime.StreamConn{
		UserID:       r.URL.Query().Get("user_id"),
		AgentID:      r.URL.Query().Get("agent_id"),
		Persona:      r.URL.Query().Get("persona"),
		OperatorRole: r.URL.Query().Get("operator_role"),
		Model:        r.URL.Query().Get("model"),
	}
	if s.authMode == AuthJWT && jwtUserID != "" {
		connCtx.UserID = jwtUserID
	}

	if err := validateStreamConn(connCtx); err != nil {
		log.Printf("[ws] stream conn validation: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	conn, err := s.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	writer := &wsConnWriter{conn: conn}

	conn.SetReadLimit(s.ReadLimit)
	const readIdle = 3 * time.Minute
	_ = conn.SetReadDeadline(time.Now().Add(readIdle))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(readIdle))
		return nil
	})

	pingCtx, stopPing := context.WithCancel(context.Background())
	defer stopPing()
	go func() {
		t := time.NewTicker(25 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-t.C:
				deadline := time.Now().Add(10 * time.Second)
				if err := writer.writeControl(websocket.PingMessage, nil, deadline); err != nil {
					return
				}
			}
		}
	}()

	ip := clientIP(r)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(readIdle))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if !s.msgRate.allow(ip) {
			log.Printf("[ws] rate limit exceeded ip=%s", ip)
			_ = writer.writeJSON(AssistantTextMessage{
				Type:      "assistant_text",
				SessionID: "",
				Text:      "Too many messages. Please wait and retry.",
			})
			continue
		}

		var req UserMessage
		if err := json.Unmarshal(data, &req); err != nil {
			_ = writer.writeJSON(AssistantTextMessage{
				Type:      "assistant_text",
				SessionID: "",
				Text:      "invalid json",
			})
			continue
		}

		if req.Type != "user_message" || req.SessionID == "" || req.Text == "" {
			_ = writer.writeJSON(AssistantTextMessage{
				Type:      "assistant_text",
				SessionID: req.SessionID,
				Text:      "expected {type:'user_message', session_id, text}",
			})
			continue
		}

		if err := validateSessionID(req.SessionID); err != nil {
			log.Printf("[ws] session_id: %v", err)
			_ = writer.writeJSON(AssistantTextMessage{
				Type:      "assistant_text",
				SessionID: req.SessionID,
				Text:      "Invalid session_id.",
			})
			continue
		}

		if err := validateUserMessageText(req.Text); err != nil {
			log.Printf("[ws] message text: %v", err)
			_ = writer.writeJSON(AssistantTextMessage{
				Type:      "assistant_text",
				SessionID: req.SessionID,
				Text:      "Message too long.",
			})
			continue
		}

		n := &notifier{w: writer}
		if err := s.Chat.ChatStream(r.Context(), req.SessionID, req.Text, n, connCtx); err != nil {
			log.Printf("[ws] ChatStream: %v", err)
			_ = writer.writeJSON(AssistantTextMessage{
				Type:      "assistant_text",
				SessionID: req.SessionID,
				Text:      clientVisibleChatError(err),
			})
		}
	}
}
