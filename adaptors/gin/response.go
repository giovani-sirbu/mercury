package ginAdaptors

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
	"strings"
)

type Data struct {
	Message string `json:"message"`
}

type ValidationErrors struct {
	Message string      `json:"message"`
	Errors  interface{} `json:"errors"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func checkTagRules(e validator.FieldError) (errMsg string) {
	tag, field, param, value := e.ActualTag(), e.Field(), e.Param(), e.Value()

	switch tag {
	case "required":
		errMsg = "This field is required"
		break
	case "email":
		errMsg = fmt.Sprintf("%q is not a valid email", value)
		break
	case "min":
		errMsg = fmt.Sprintf("%s must have length greater than %v chars", field, param)
	case "max":
		errMsg = fmt.Sprintf("%s must have length less than %v chars", field, param)
		break
	case "containsany":
		errMsg = fmt.Sprintf("%s must contain at least one of the following chars: %v", field, param)
		break
	default:
		errMsg = "failed to validate field"
	}

	return
}

func Response(c *gin.Context, statusCode int, data interface{}) error {
	c.JSON(statusCode, data)
	return nil
}

func MessageResponse(c *gin.Context, statusCode int, message string) error {
	return Response(c, statusCode, Data{
		Message: message,
	})
}

func ValidationResponse(c *gin.Context, _err error) {
	var errors []ValidationError

	switch err := _err.(type) {
	case validator.ValidationErrors:
		for _, e := range err {
			errors = append(errors, ValidationError{
				Field:   strings.ToLower(e.Field()),
				Message: checkTagRules(e),
			})
		}
	default:
		errors = append(errors, ValidationError{
			Field:   "all",
			Message: "invalid payload",
		})
	}

	err := Response(c, http.StatusUnprocessableEntity, ValidationErrors{
		Message: "VALIDATION_ERROR",
		Errors:  errors,
	})

	if err != nil {
		return
	}
}
