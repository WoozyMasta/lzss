package lzss

import "errors"

// Package errors. Use errors.New for static messages, fmt.Errorf when values are needed.
var (
	ErrInputTooShort    = errors.New("not enough data for checksum")
	ErrUnexpectedEOF    = errors.New("unexpected end of input while reading flags")
	ErrUnexpectedEOFBit = errors.New("unexpected end of input inside flags block")
	ErrEmptyInput       = errors.New("input is empty")
)
