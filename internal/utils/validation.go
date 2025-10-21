package utils

import (
    "regexp"
    "unicode"
)

func ContainsUppercase(s string) bool {
    for _, r := range s {
        if unicode.IsUpper(r) {
            return true
        }
    }
    return false
}

func ContainsLowercase(s string) bool {
    for _, r := range s {
        if unicode.IsLower(r) {
            return true
        }
    }
    return false
}

func ContainsNumber(s string) bool {
    for _, r := range s {
        if unicode.IsDigit(r) {
            return true
        }
    }
    return false
}

func ContainsSpecialChar(s string) bool {
    for _, r := range s {
        if unicode.IsPunct(r) || unicode.IsSymbol(r) {
            return true
        }
    }
    return false
}

func IsValidUsername(username string) bool {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, username)
    return matched
}

func IsValidEmail(email string) bool {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
    return matched
}