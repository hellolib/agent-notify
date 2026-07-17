package cli

import (
	"errors"
	"io"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/hellolib/agent-notify/internal/i18n"
)

// ErrCancelled 表示用户按 Ctrl+C 取消操作
var ErrCancelled = errors.New("用户取消")

type PromptOption struct {
	Label string
	Value string
}

type Prompter interface {
	Select(message string, options []PromptOption, defaultValue string) (string, error)
	MultiSelect(message string, options []PromptOption, defaults []string) ([]string, error)
	Confirm(message string, defaultValue bool) (bool, error)
	Input(message, defaultValue string) (string, error)
}

var newPrompter = func(streams Streams) (Prompter, error) {
	return newSurveyPrompter(streams)
}

type surveyPrompter struct {
	in  terminal.FileReader
	out terminal.FileWriter
	err io.Writer
}

func newSurveyPrompter(streams Streams) (Prompter, error) {
	in, ok := streams.Stdin.(terminal.FileReader)
	if !ok {
		return nil, errors.New("interactive TUI requires a terminal stdin")
	}
	out, ok := streams.Stdout.(terminal.FileWriter)
	if !ok {
		return nil, errors.New("interactive TUI requires a terminal stdout")
	}
	return &surveyPrompter{
		in:  in,
		out: out,
		err: streams.Stderr,
	}, nil
}

func (p *surveyPrompter) Select(message string, options []PromptOption, defaultValue string) (string, error) {
	labels := make([]string, 0, len(options))
	labelToValue := make(map[string]string, len(options))
	defaultLabel := ""

	for _, option := range options {
		labels = append(labels, option.Label)
		labelToValue[option.Label] = option.Value
		if option.Value == defaultValue {
			defaultLabel = option.Label
		}
	}

	choice := defaultLabel
	prompt := &survey.Select{
		Message: message,
		Options: labels,
		Default: defaultLabel,
	}
	if err := survey.AskOne(prompt, &choice, p.askOpts()...); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return "", ErrCancelled
		}
		return "", err
	}
	return labelToValue[choice], nil
}

func (p *surveyPrompter) MultiSelect(message string, options []PromptOption, defaults []string) ([]string, error) {
	labels := make([]string, 0, len(options))
	labelToValue := make(map[string]string, len(options))
	defaultLabels := make([]string, 0, len(defaults))
	defaultSet := make(map[string]struct{}, len(defaults))
	for _, value := range defaults {
		defaultSet[value] = struct{}{}
	}

	for _, option := range options {
		labels = append(labels, option.Label)
		labelToValue[option.Label] = option.Value
		if _, ok := defaultSet[option.Value]; ok {
			defaultLabels = append(defaultLabels, option.Label)
		}
	}

	// 必须为空切片：survey 在写入 []string 时会把选项追加到目标切片末尾，若此处预填默认值，
	// 用户取消勾选后结果仍会包含旧项，导致配置文件无法正确反映取消。
	selectedLabels := []string{}
	prompt := &survey.MultiSelect{
		Message: message,
		Options: labels,
		Default: defaultLabels,
	}
	if err := survey.AskOne(prompt, &selectedLabels, p.askOpts()...); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return nil, ErrCancelled
		}
		return nil, err
	}

	selectedValues := make([]string, 0, len(selectedLabels))
	for _, label := range selectedLabels {
		selectedValues = append(selectedValues, labelToValue[label])
	}
	return selectedValues, nil
}

func (p *surveyPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	answer := defaultValue
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultValue,
	}
	if err := survey.AskOne(prompt, &answer, p.askOpts()...); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return false, ErrCancelled
		}
		return false, err
	}
	return answer, nil
}

func (p *surveyPrompter) Input(message, defaultValue string) (string, error) {
	answer := defaultValue
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}
	if err := survey.AskOne(prompt, &answer, p.askOpts()...); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return "", ErrCancelled
		}
		return "", err
	}
	return answer, nil
}

func (p *surveyPrompter) askOpts() []survey.AskOpt {
	return []survey.AskOpt{
		survey.WithStdio(p.in, p.out, p.err),
		survey.WithPageSize(10),
		// survey.Select 会把可打印字符（含空格）当 type-to-filter。
		// 默认 filter 是 strings.Contains；中文菜单项通常不含空格，
		// 按空格会滤空列表，界面像“菜单消失”，只能 Backspace 才能恢复。
		// 把纯空白 filter 视为无筛选，避免误触空格把选项滤没。
		survey.WithFilter(selectFilter),
		survey.WithIcons(func(icons *survey.IconSet) {
			icons.Question.Text = "?"
			icons.Help.Text = i18n.T("prompt.help.multiselect")
			icons.MarkedOption.Text = "[✓]"   // 多选选中项使用对号
			icons.UnmarkedOption.Text = "[ ]" // 多选未选中项
		}),
	}
}

// selectFilter 与 survey 默认 filter 一致（大小写不敏感子串匹配），
// 但忽略纯空白输入，避免 Select 菜单在按空格后被滤空。
func selectFilter(filter, value string, _ int) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(filter))
}
