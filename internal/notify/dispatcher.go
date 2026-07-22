package notify

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/state"
)

type Dispatcher struct {
	store   *state.Store
	window  time.Duration
	senders []Sender
}

func NewDispatcher(store *state.Store, window time.Duration, senders ...Sender) *Dispatcher {
	return &Dispatcher{
		store:   store,
		window:  window,
		senders: senders,
	}
}

func (d *Dispatcher) SendAll(ctx context.Context, msg Message) error {
	var errs []string
	for _, sender := range d.senders {
		now := time.Now()
		key := dedupeKey(msg, sender.Name(), os.Getppid())
		allow, err := d.store.ReserveSend(key, d.window, now)
		if err != nil {
			return err
		}
		if !allow {
			continue
		}
		if err := sender.Send(ctx, msg); err != nil {
			_ = d.store.ClearReservation(key)
			errs = append(errs, fmt.Sprintf("%s: %v", sender.Name(), err))
			continue
		}
		if err := d.store.MarkSent(key, d.window, now); err != nil {
			return err
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.New(strings.Join(errs, "; "))
}

// dedupeKey 构造去重键：agent \x00 session \x00 event \x00 contentHash \x00 sender。
// contentHash 用 fnv-1a-64 对 Title+Body 取哈希，使去重精确到「同一条内容」。
// SessionID 为空时用 ppid 兜底，避免多实例塌缩到同一键而误吞。
func dedupeKey(msg Message, senderName string, ppid int) string {
	session := msg.SessionID
	if session == "" {
		session = "ppid-" + strconv.Itoa(ppid)
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(msg.Title))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(msg.Body))
	content := strconv.FormatUint(h.Sum64(), 16)
	return strings.Join([]string{msg.Agent, session, msg.Event, content, senderName}, "\x00")
}
