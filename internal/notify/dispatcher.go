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

// SendAll 对 store 故障 fail-open:去重是尽力而为的降噪层,不是正确性保障。
// 本工具的存在意义是「agent 需要你时你一定知道」——漏发一条 permission_required
// 的代价(agent 干等)远大于重复一条的代价,因此 store 出错时照发不误,
// 错误仅记入返回值供调用方写日志,绝不因 store 中止剩余 sender。
func (d *Dispatcher) SendAll(ctx context.Context, msg Message) error {
	var errs []string
	for _, sender := range d.senders {
		now := time.Now()
		key := dedupeKey(msg, sender.Name(), os.Getppid())
		allow, err := d.store.ReserveSend(key, d.window, now)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: dedupe store: %v", sender.Name(), err))
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
			errs = append(errs, fmt.Sprintf("%s: mark sent: %v", sender.Name(), err))
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
