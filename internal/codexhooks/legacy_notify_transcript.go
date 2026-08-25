package codexhooks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	legacyNotifyCompletionWait = 2 * time.Second
	legacyNotifyPollInterval   = 100 * time.Millisecond
	legacyTranscriptMaxAge     = 48 * time.Hour
	legacyThreadStartTolerance = 10 * time.Second
	legacyTranscriptMaxLine    = 16 * 1024 * 1024
)

type legacyTurnState uint8

const (
	legacyTurnUnknown legacyTurnState = iota
	legacyTurnRunning
	legacyTurnComplete
)

type legacyTurnCompletion struct {
	State            legacyTurnState
	LastAgentMessage string
	TranscriptPath   string
}

type legacyTranscriptLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type             string `json:"type"`
		CWD              string `json:"cwd"`
		TurnID           string `json:"turn_id"`
		LastAgentMessage string `json:"last_agent_message"`
	} `json:"payload"`
}

type legacyTranscriptTask struct {
	StartedAt        time.Time
	Complete         bool
	LastAgentMessage string
}

type legacyTranscriptCandidate struct {
	Path    string
	ModTime time.Time
}

func isVSCodeNotifyClient(client string) bool {
	normalized := strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(client))
	return strings.Contains(normalized, "vscode")
}

// VS Code can start a persisted task while a stale app-server agent turn is still finishing.
// The legacy event does not expose task state, so use the transcript lifecycle as a fail-open
// guard: a matching task_started without task_complete is not a completed user task.
func waitForLegacyNotifyCompletion(
	ctx context.Context,
	codexHome string,
	p legacyNotifyPayload,
	wait time.Duration,
) (legacyTurnCompletion, error) {
	deadline := time.Now().Add(wait)
	completion, err := inspectLegacyNotifyTranscripts(codexHome, p, time.Now())
	if err != nil || completion.State != legacyTurnRunning {
		return completion, err
	}
	last := completion
	threadStartedAt, hasThreadStartedAt := uuidV7Time(p.ThreadID)

	for {
		if !time.Now().Before(deadline) {
			return last, nil
		}

		timer := time.NewTimer(legacyNotifyPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return legacyTurnCompletion{}, ctx.Err()
		case <-timer.C:
		}

		completion, err = inspectLegacyTranscript(
			last.TranscriptPath,
			p.CWD,
			p.TurnID,
			threadStartedAt,
			hasThreadStartedAt,
		)
		if err != nil {
			return legacyTurnCompletion{}, err
		}
		if completion.State == legacyTurnComplete {
			return completion, nil
		}
		if completion.State == legacyTurnRunning {
			last = completion
		}
	}
}

func inspectLegacyNotifyTranscripts(
	codexHome string,
	p legacyNotifyPayload,
	now time.Time,
) (legacyTurnCompletion, error) {
	candidates, err := recentLegacyTranscripts(filepath.Join(codexHome, "sessions"), now)
	if err != nil {
		return legacyTurnCompletion{}, err
	}

	threadStartedAt, hasThreadStartedAt := uuidV7Time(p.ThreadID)
	running := legacyTurnCompletion{State: legacyTurnUnknown}
	var firstErr error
	for _, candidate := range candidates {
		completion, err := inspectLegacyTranscript(
			candidate.Path,
			p.CWD,
			p.TurnID,
			threadStartedAt,
			hasThreadStartedAt,
		)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if completion.State == legacyTurnComplete {
			return completion, nil
		}
		if completion.State == legacyTurnRunning && running.State == legacyTurnUnknown {
			running = completion
		}
	}
	if running.State == legacyTurnUnknown && firstErr != nil {
		return legacyTurnCompletion{}, firstErr
	}
	return running, nil
}

func recentLegacyTranscripts(root string, now time.Time) ([]legacyTranscriptCandidate, error) {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	cutoff := now.Add(-legacyTranscriptMaxAge)
	var candidates []legacyTranscriptCandidate
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			return nil
		}
		candidates = append(candidates, legacyTranscriptCandidate{
			Path:    path,
			ModTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ModTime.After(candidates[j].ModTime)
	})
	return candidates, nil
}

func inspectLegacyTranscript(
	path string,
	workspace string,
	turnID string,
	threadStartedAt time.Time,
	hasThreadStartedAt bool,
) (legacyTurnCompletion, error) {
	f, err := os.Open(path)
	if err != nil {
		return legacyTurnCompletion{}, err
	}
	defer f.Close()

	var transcriptWorkspace string
	tasks := make(map[string]*legacyTranscriptTask)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), legacyTranscriptMaxLine)
	for scanner.Scan() {
		var line legacyTranscriptLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			// Codex may be appending the final line while this detached handler reads it.
			continue
		}
		switch {
		case line.Type == "session_meta":
			transcriptWorkspace = line.Payload.CWD
		case line.Type == "event_msg" && line.Payload.Type == "task_started":
			task := tasks[line.Payload.TurnID]
			if task == nil {
				task = &legacyTranscriptTask{}
				tasks[line.Payload.TurnID] = task
			}
			task.StartedAt, _ = time.Parse(time.RFC3339Nano, line.Timestamp)
		case line.Type == "event_msg" && line.Payload.Type == "task_complete":
			task := tasks[line.Payload.TurnID]
			if task == nil {
				task = &legacyTranscriptTask{}
				tasks[line.Payload.TurnID] = task
			}
			task.Complete = true
			task.LastAgentMessage = line.Payload.LastAgentMessage
		}
	}
	if err := scanner.Err(); err != nil {
		return legacyTurnCompletion{}, err
	}
	if !sameLegacyWorkspace(transcriptWorkspace, workspace) {
		return legacyTurnCompletion{State: legacyTurnUnknown}, nil
	}

	if task := tasks[turnID]; task != nil {
		return completionFromLegacyTask(path, task), nil
	}
	if !hasThreadStartedAt {
		return legacyTurnCompletion{State: legacyTurnUnknown}, nil
	}

	var correlated *legacyTranscriptTask
	closest := legacyThreadStartTolerance + time.Nanosecond
	for _, task := range tasks {
		if task.StartedAt.IsZero() {
			continue
		}
		delta := task.StartedAt.Sub(threadStartedAt)
		if delta < 0 {
			delta = -delta
		}
		if delta <= legacyThreadStartTolerance && delta < closest {
			correlated = task
			closest = delta
		}
	}
	if correlated == nil {
		return legacyTurnCompletion{State: legacyTurnUnknown}, nil
	}
	return completionFromLegacyTask(path, correlated), nil
}

func completionFromLegacyTask(path string, task *legacyTranscriptTask) legacyTurnCompletion {
	state := legacyTurnRunning
	if task.Complete {
		state = legacyTurnComplete
	}
	return legacyTurnCompletion{
		State:            state,
		LastAgentMessage: task.LastAgentMessage,
		TranscriptPath:   path,
	}
}

func sameLegacyWorkspace(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func uuidV7Time(id string) (time.Time, bool) {
	compact := strings.ReplaceAll(id, "-", "")
	if len(compact) < 13 || compact[12] != '7' {
		return time.Time{}, false
	}
	milliseconds, err := strconv.ParseInt(compact[:12], 16, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(milliseconds).UTC(), true
}
