package request

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type (
	contextWrapperService interface {
		Bind(data any) error
	}

	contextWrapper struct {
		Context   echo.Context
		validator *validator.Validate
	}
)

func ContextWrapper(ctx echo.Context) contextWrapperService {
	return &contextWrapper{
		Context:   ctx,
		validator: validator.New(),
	}
}

func (c contextWrapper) Bind(data any) error {
	if err := c.Context.Bind(data); err != nil {
		log.Printf("Error: Bind data failed: %s", err.Error())
		return errors.New("error: bad request")
	}

	if err := c.validator.Struct(data); err != nil {
		log.Printf("Error: Validate data failed: %s", err.Error())
		return formatValidationError(err)
	}

	return nil
}

func formatValidationError(err error) error {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		// ไม่ใช่ error จาก validate ปกติ → ตอบกลาง ๆ กันข้อมูลภายในหลุด
		return errors.New("error: invalid request")
	}

	messages := make([]string, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		rule := fieldErr.Tag()
		if fieldErr.Param() != "" {
			rule = fmt.Sprintf("%s=%s", fieldErr.Tag(), fieldErr.Param())
		}
		messages = append(messages, fmt.Sprintf("%s: failed on '%s'", strings.ToLower(fieldErr.Field()), rule))
	}

	return errors.New("error: " + strings.Join(messages, ", "))
}
