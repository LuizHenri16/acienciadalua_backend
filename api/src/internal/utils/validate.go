package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Messages maps "<field>.<tag>" to a custom message. When a key is absent the
// message is generated in class-validator's wording.
type Messages map[string]string

var validatorInstance = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return field.Name
		}
		return name
	})
	return v
}

// Validate checks a struct and returns class-validator style messages.
func Validate(s any, custom Messages) []string {
	err := validatorInstance.Struct(s)
	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	if !asValidationErrors(err, &validationErrors) {
		return []string{err.Error()}
	}

	messages := make([]string, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		field := fieldName(fieldErr)
		if custom != nil {
			if message, ok := custom[field+"."+fieldErr.Tag()]; ok {
				messages = append(messages, message)
				continue
			}
		}
		messages = append(messages, defaultMessage(field, fieldErr))
	}
	return messages
}

func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	validationErrors, ok := err.(validator.ValidationErrors)
	if ok {
		*target = validationErrors
	}
	return ok
}

func fieldName(fieldErr validator.FieldError) string {
	namespace := fieldErr.Namespace()
	parts := strings.Split(namespace, ".")
	if len(parts) > 1 {
		return strings.Join(parts[1:], ".")
	}
	return fieldErr.Field()
}

func defaultMessage(field string, fieldErr validator.FieldError) string {
	param := fieldErr.Param()

	switch fieldErr.Tag() {
	case "required":
		return field + " should not be empty"
	case "email":
		return field + " must be an email"
	case "min":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("%s must be longer than or equal to %s characters", field, param)
		}
		return fmt.Sprintf("%s must not be less than %s", field, param)
	case "max":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("%s must be shorter than or equal to %s characters", field, param)
		}
		return fmt.Sprintf("%s must not be greater than %s", field, param)
	case "gte":
		return fmt.Sprintf("%s must not be less than %s", field, param)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, param)
	case "lte":
		return fmt.Sprintf("%s must not be greater than %s", field, param)
	case "oneof":
		values := strings.Join(strings.Fields(param), ", ")
		return fmt.Sprintf("%s must be one of the following values: %s", field, values)
	case "boolean":
		return field + " must be a boolean value"
	case "numeric":
		return field + " must be a number conforming to the specified constraints"
	case "url":
		return field + " must be a URL address"
	case "uuid":
		return field + " must be a UUID"
	case "len":
		return fmt.Sprintf("%s must be equal to %s characters", field, param)
	default:
		return field + " is invalid"
	}
}
