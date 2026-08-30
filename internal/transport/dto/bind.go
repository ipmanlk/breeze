package dto

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func BindAndValidate(r *http.Request, obj any) error {
	if err := render.DecodeJSON(r.Body, obj); err != nil {
		return err
	}
	return validate.Struct(obj)
}
