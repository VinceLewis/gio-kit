package form

import (
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"
)

func MinLength(minimum int) Validator {
	return func(value string) (bool, string) {
		if utf8.RuneCountInString(value) < minimum {
			return false, fmt.Sprintf("Must be at least %d characters", minimum)
		}
		return true, ""
	}
}

func MaxLength(maximum int) Validator {
	return func(value string) (bool, string) {
		if utf8.RuneCountInString(value) > maximum {
			return false, fmt.Sprintf("Must be at most %d characters", maximum)
		}
		return true, ""
	}
}

func Pattern(expression, message string) (Validator, error) {
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return nil, err
	}
	return func(value string) (bool, string) {
		if !compiled.MatchString(value) {
			return false, message
		}
		return true, ""
	}, nil
}

func NumberRange(minimum, maximum float64) Validator {
	return func(value string) (bool, string) {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || number < minimum || number > maximum {
			return false, fmt.Sprintf("Must be a number from %g to %g", minimum, maximum)
		}
		return true, ""
	}
}
