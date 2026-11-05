package request

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

type BadRequestError struct {
	Message string
}

func (e BadRequestError) Error() string {
	return e.Message
}

func DecodeJSONArray(w http.ResponseWriter, r *http.Request, dst any) error {
	return DecodeJSON(w, r, dst, jsontext.KindBeginArray)
}

func DecodeJSONObject(w http.ResponseWriter, r *http.Request, dst any) error {
	return DecodeJSON(w, r, dst, jsontext.KindBeginObject)
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, expectedFirstToken jsontext.Kind) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	opts := []json.Options{
		json.RejectUnknownMembers(true),
	}

	err := json.UnmarshalRead(r.Body, dst, opts...)
	if err != nil {
		var synErr *jsontext.SyntacticError
		var semErr *json.SemanticError
		var maxBytesErr *http.MaxBytesError

		switch {
		case errors.As(err, &maxBytesErr):
			return convertMaxBytesErrorToBadRequest(maxBytesErr)

		case errors.As(err, &synErr):
			return convertSyntacticErrorToBadRequest(synErr)

		case errors.As(err, &semErr) && isClientSemanticError(semErr, expectedFirstToken):
			return convertSemanticErrorToBadRequest(semErr)

		default:
			return err
		}
	}

	return nil
}

func isClientSemanticError(semErr *json.SemanticError, expectedFirstToken jsontext.Kind) bool {
	if semErr.Err != nil {
		msgs := []string{
			"ambiguous object member name",
			"cannot derive concrete type for nil interface with finite type set",
			"Go struct has no exported fields",
			"value must be passed as a non-nil pointer reference",
		}
		if slices.Contains(msgs, semErr.Err.Error()) {
			return false
		}
	}

	if semErr.ByteOffset == 0 && semErr.JSONKind != expectedFirstToken {
		return true
	}

	if semErr.ByteOffset > 0 {
		return true
	}

	return false
}

func convertSemanticErrorToBadRequest(semErr *json.SemanticError) *BadRequestError {
	if errors.Is(semErr, json.ErrUnknownName) {
		return &BadRequestError{
			Message: fmt.Sprintf("request body contains unknown key %s", semErr.JSONPointer),
		}
	}

	var buf strings.Builder

	pointer := string(semErr.JSONPointer)
	if pointer == "" {
		pointer = "request body"
	}
	fmt.Fprintf(&buf, "%s contains an unsupported JSON type or invalid value", pointer)

	offset := semErr.ByteOffset
	if offset > 0 {
		fmt.Fprintf(&buf, " at position %d", offset)
	}

	return &BadRequestError{Message: buf.String()}
}

func convertSyntacticErrorToBadRequest(synErr *jsontext.SyntacticError) *BadRequestError {
	if errors.Is(synErr, io.ErrUnexpectedEOF) && synErr.ByteOffset == 0 {
		return &BadRequestError{Message: "request body must not be empty"}
	}

	var buf strings.Builder

	pointer := string(synErr.JSONPointer)
	if pointer == "" {
		pointer = "request body"
	}
	fmt.Fprintf(&buf, "%s contains malformed JSON", pointer)

	offset := synErr.ByteOffset
	if offset > 0 {
		fmt.Fprintf(&buf, " at position %d", offset)
	}

	detail := synErr.Err.Error()

	if detail == "unexpected EOF" {
		detail = "unexpected end of JSON"
	}
	fmt.Fprintf(&buf, ": %s", detail)

	return &BadRequestError{Message: buf.String()}
}

func convertMaxBytesErrorToBadRequest(maxBytesErr *http.MaxBytesError) *BadRequestError {
	return &BadRequestError{
		Message: fmt.Sprintf("request body must not be larger than %d bytes", maxBytesErr.Limit),
	}
}
