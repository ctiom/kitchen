package kitchen

type DefaultWebWrapper struct{}

type defaultDataWrapperErr struct {
	Code    int               `required:"true" json:",omitempty"`
	Message string            `required:"true" json:",omitempty"`
	Extra   map[string]string `json:",omitempty"`
}

type defaultDataWrapper struct {
	Data  any                    `required:"true" json:",omitempty"`
	Error *defaultDataWrapperErr `required:"true" json:",omitempty"`
}

func (d DefaultWebWrapper) WrapWebOutput(output any, err error) (any, int) {
	res := defaultDataWrapper{}
	if err != nil {
		res.Error = &defaultDataWrapperErr{}
		if webErr, ok := err.(*WebErr); ok {
			res.Error.Code = int(webErr.ErrorCode)
			res.Error.Message = webErr.Err.Error()
			res.Error.Extra = webErr.Extra
			return res, webErr.HttpStatus
		} else {
			res.Error.Message = err.Error()
		}
	} else {
		res.Data = output
	}
	return res, 200
}
