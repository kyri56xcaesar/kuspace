package model

// helpers for Job.ValidateForm (moved from utils with the model)

import (
	"strconv"
	"strings"
)

func parseCPU(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}

	return strconv.ParseFloat(s, 64)
}

func parseMi(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, "Mi")
	if s == "" {
		return 0, nil
	}

	return strconv.Atoi(s)
}

func parseGi(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, "Gi")
	if s == "" {
		return 0, nil
	}

	return strconv.ParseFloat(s, 64)
}

func appendIfMissing(s, suffix string) string {
	if !strings.HasSuffix(s, suffix) {
		return s + suffix
	}

	return s
}
