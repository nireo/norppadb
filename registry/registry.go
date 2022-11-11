package registry

import (
	"net"

	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
)

type Config struct {
	NodeName       string
	BindAddr       string
	Tags           map[string]string
	StartJoinAddrs []string
}

type Handler interface {
	Join(id, addr string) error
	Leave(id string) error
}

type Registry struct {
	Config
	handler Handler
	serf    *serf.Serf
	events  chan serf.Event
	logger  *zap.Logger
}

func New(handler Handler, config Config) (*Registry, error) {
	r := &Registry{
		Config: config, handler: handler, logger: zap.L().Named("registry"),
	}

	if err := r.setupSerf(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Registry) setupSerf() error {
	addr, err := net.ResolveTCPAddr("tcp", r.BindAddr)
	if err != nil {
		return err
	}

	config := serf.DefaultConfig()
	config.Init() // allocate subdata structures

	config.MemberlistConfig.BindAddr = addr.IP.String()
	config.MemberlistConfig.BindPort = addr.Port

	r.events = make(chan serf.Event)
	config.EventCh = r.events
	config.Tags = r.Tags
	config.NodeName = r.NodeName

	r.serf, err = serf.Create(config)
	if err != nil {
		return err
	}

	go r.eventHandler()
	if r.StartJoinAddrs != nil {
		if _, err := r.serf.Join(r.StartJoinAddrs, true); err != nil {
			return err
		}
	}

	return nil
}

// eventHandler handles all of the important serf events.
func (r *Registry) eventHandler() {
	for e := range r.events {
		switch e.EventType() {
		case serf.EventMemberJoin:
			for _, member := range e.(serf.MemberEvent).Members {
				if r.isLocal(member) {
					continue
				}
				r.handleJoin(member)
			}
		case serf.EventMemberLeave:
			for _, member := range e.(serf.MemberEvent).Members {
				if r.isLocal(member) {
					continue
				}
				r.handleLeave(member)
			}
		}
	}
}

func (r *Registry) handleJoin(member serf.Member) {
	if err := r.handler.Join(member.Name, member.Tags["addr"]); err != nil {
		r.logError(err, "failed to join", member)
	}
}

func (r *Registry) handleLeave(member serf.Member) {
	if err := r.handler.Leave(member.Name); err != nil {
		r.logError(err, "failed to leave", member)
	}
}

// isLocal checks wheter the given Serf member is the local member
// by checking the members' names
func (r *Registry) isLocal(member serf.Member) bool {
	return r.serf.LocalMember().Name == member.Name
}

// Members returns a point-in-time snapshot of the cluster's members.
func (r *Registry) Members() []serf.Member {
	return r.serf.Members()
}

// Leave tells this member to leave the cluster.
func (r *Registry) Leave() error {
	return r.serf.Leave()
}

func (r *Registry) logError(err error, msg string, member serf.Member) {
	r.logger.Error(
		msg,
		zap.Error(err),
		zap.String("name", member.Name),
		zap.String("addr", member.Tags["addr"]),
	)
}
