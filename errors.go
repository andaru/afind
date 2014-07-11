package afind

// Error types

type errorMsg struct {
	msg string
}

func (e errorMsg) Error() string {
	return e.msg
}

type errorMsgAction struct {
	errorMsg
	action string
}

func (e errorMsgAction) Error() string {
	if e.action != "" {
		return e.action + `:` + e.errorMsg.msg
	}
	return e.errorMsg.msg
}

func NewApiError(action, message string) error {
	return errorMsgAction{errorMsg{message}, action}
}


















