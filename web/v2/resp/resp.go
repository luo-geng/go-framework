package resp

type Resp[T any] struct {
	Ok   bool   `json:"ok"`
	Data T      `json:"data,omitempty"`
	Msg  string `json:"msg"`
}

type PageResp[T any] struct {
	Resp[T]
	Total int64 `json:"total"`
}

const (
	MsgOk = "success"
)

func Success[T any](data T) Resp[T] {
	return Resp[T]{
		Ok:   true,
		Data: data,
		Msg:  MsgOk,
	}
}

func SuccessNoData() Resp[any] {
	return Resp[any]{
		Ok:  true,
		Msg: MsgOk,
	}
}

func Fail(msg string) Resp[any] {
	return Resp[any]{
		Ok:  false,
		Msg: msg,
	}
}

func FailWithError(err error) Resp[any] {
	return Fail(err.Error())
}
