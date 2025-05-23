package binding

import (
	"net/http"
	"net/url"

	"github.com/yola1107/kratos/v2/encoding"
	"github.com/yola1107/kratos/v2/encoding/form"
	"github.com/yola1107/kratos/v2/errors"
)

// BindQuery bind vars parameters to target.
func BindQuery(vars url.Values, target any) error {
	if err := encoding.GetCodec(form.Name).Unmarshal([]byte(vars.Encode()), target); err != nil {
		return errors.BadRequest(errors.CodecReason, err.Error())
	}
	return nil
}

// BindForm bind form parameters to target.
func BindForm(req *http.Request, target any) error {
	if err := req.ParseForm(); err != nil {
		return err
	}
	if err := encoding.GetCodec(form.Name).Unmarshal([]byte(req.Form.Encode()), target); err != nil {
		return errors.BadRequest(errors.CodecReason, err.Error())
	}
	return nil
}
