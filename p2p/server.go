package p2p

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
)

type GameVariant uint8

func (gv GameVariant) String() string {
	switch gv {
	case TexasHoldem:
		return "TEXAS HOLDEM"
	case Other:
		return "other"
	default:
		return "unknown"
	}
}

const (
	TexasHoldem GameVariant = iota
	Other
)

type ServerConfig struct {
	Version     string
	ListenAddr  string
	GameVariant GameVariant
}

type Server struct {
	ServerConfig

	handler   Handler
	transport *TCPTransport
	peers     map[net.Addr]*Peer
	addPeer   chan *Peer
	delPeer   chan *Peer
	msgCh     chan *Message
}

func NewServer(cfg ServerConfig) *Server {

	s := &Server{
		handler:      &DefaultHandler{},
		ServerConfig: cfg,
		peers:        make(map[net.Addr]*Peer),
		addPeer:      make(chan *Peer),
		delPeer:      make(chan *Peer),
		msgCh:        make(chan *Message),
	}
	tr := NewTCPTransport(s.ListenAddr)
	s.transport = tr
	tr.AddPeer = s.addPeer
	tr.DelPeer = s.delPeer
	return s
}

func (s *Server) Start() {
	go s.loop()

	fmt.Printf("game server running on port %s\n", s.ListenAddr)
	logrus.WithFields(logrus.Fields{
		"port":   s.ListenAddr,
		"varian": s.GameVariant,
	}).Info("started new game server")
	s.transport.ListenAndAccept()

}

// TODO: Right now we have some redundant code in registering new peers to the game network.
// maybe construct a new peer and handshake protocol after registering a plain connection?
func (s *Server) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	peer := &Peer{
		conn: conn,
	}
	s.addPeer <- peer

	return peer.Send([]byte(s.Version))
}

func (s *Server) loop() {
	for {
		select {
		case peer := <-s.delPeer:

			logrus.WithFields(logrus.Fields{
				"addr": peer.conn.RemoteAddr(),
			}).Info("player disconnected")

			delete(s.peers, peer.conn.RemoteAddr())

		case peer := <-s.addPeer:

			//TODO: check max players and other game state logic
			go peer.ReadLoop(s.msgCh)
			logrus.WithFields(logrus.Fields{
				"addr": peer.conn.RemoteAddr(),
			}).Info("new player connected")

			s.peers[peer.conn.RemoteAddr()] = peer

		case msg := <-s.msgCh:
			if err := s.handler.HandleMessage(msg); err != nil {
				panic(err)
			}
		}
	}
}
