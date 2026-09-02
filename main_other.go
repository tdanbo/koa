//go:build !linux

package main

// preventDoubleDecorations only applies to Linux/GTK; koa's native Windows
// frameless window has no equivalent decoration-negotiation issue.
func preventDoubleDecorations() {}
