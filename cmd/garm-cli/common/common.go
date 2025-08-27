// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/nbutton23/zxcvbn-go"

	"github.com/cloudbase/garm-provider-common/util"
)

func PromptPassword(label string, compareTo string) (string, error) {
	if label == "" {
		label = "Password"
	}
	validate := func(input string) error {
		passwordStenght := zxcvbn.PasswordStrength(input, nil)
		if passwordStenght.Score < 4 {
			return errors.New("password is too weak")
		}
		if compareTo != "" && compareTo != input {
			return errors.New("passwords do not match")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    label,
		Validate: validate,
		Mask:     '*',
	}
	result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}

func PromptString(label string, a ...interface{}) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New("empty input not allowed")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    fmt.Sprintf(label, a...),
		Validate: validate,
	}
	result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}

func PrintWebsocketMessage(_ int, msg []byte) error {
	fmt.Println(util.SanitizeLogEntry(string(msg)))
	return nil
}

type LogFormatter struct {
	MinLevel         string
	AttributeFilters map[string]string
	EnableColor      bool
}

type LogRecord struct {
	Time  string                 `json:"time"`
	Level string                 `json:"level"`
	Msg   string                 `json:"msg"`
	Attrs map[string]interface{} `json:",inline"`
}

// Color codes for different log levels
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorGray    = "\033[90m"
)

func (lf *LogFormatter) colorizeLevel(level string) string {
	if !lf.EnableColor {
		return level
	}

	levelUpper := strings.TrimSpace(strings.ToUpper(level))
	switch levelUpper {
	case "ERROR":
		return ColorRed + level + ColorReset
	case "WARN", "WARNING":
		return ColorYellow + level + ColorReset
	case "INFO":
		return ColorBlue + level + ColorReset
	case "DEBUG":
		return ColorMagenta + level + ColorReset
	default:
		return level
	}
}

func (lf *LogFormatter) shouldFilterLevel(level string) bool {
	if lf.MinLevel == "" {
		return false
	}

	levelMap := map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	minLevelNum, exists := levelMap[strings.ToUpper(lf.MinLevel)]
	if !exists {
		return false
	}

	currentLevelNum, exists := levelMap[strings.ToUpper(level)]
	if !exists {
		return false
	}

	return currentLevelNum < minLevelNum
}

func (lf *LogFormatter) matchesAttributeFilters(attrs map[string]interface{}, msg string) bool {
	if len(lf.AttributeFilters) == 0 {
		return true
	}

	for key, expectedValue := range lf.AttributeFilters {
		// Special handling for message filtering
		if key == "msg" {
			if strings.Contains(msg, expectedValue) {
				return true
			}
		}

		// Regular attribute filtering
		actualValue, exists := attrs[key]
		if exists {
			actualStr := fmt.Sprintf("%v", actualValue)
			if actualStr == expectedValue {
				return true
			}
		}
	}

	return false
}

func (lf *LogFormatter) FormatWebsocketMessage(_ int, msg []byte) error {
	// Try to parse as JSON log record
	var logRecord LogRecord
	err := json.Unmarshal(msg, &logRecord)
	if err != nil {
		// If it's not JSON, print as-is (sanitized)
		_, err = fmt.Println(util.SanitizeLogEntry(string(msg)))
		return err
	}

	// Apply level filtering
	if lf.shouldFilterLevel(logRecord.Level) {
		return nil
	}

	// Parse additional attributes from the JSON
	var fullRecord map[string]interface{}
	if err := json.Unmarshal(msg, &fullRecord); err == nil {
		// Remove standard fields and keep only attributes
		delete(fullRecord, "time")
		delete(fullRecord, "level")
		delete(fullRecord, "msg")
		logRecord.Attrs = fullRecord
	}

	// Apply attribute filtering
	if !lf.matchesAttributeFilters(logRecord.Attrs, logRecord.Msg) {
		return nil
	}

	// Format timestamp to fixed width
	timeStr := logRecord.Time
	if t, err := time.Parse(time.RFC3339Nano, logRecord.Time); err == nil {
		timeStr = t.Format("2006-01-02 15:04:05.000")
	}

	// Format log level to fixed width (5 characters)
	levelStr := lf.colorizeLevel(fmt.Sprintf("%-5s", strings.ToUpper(logRecord.Level)))

	// Highlight message if it matches a msg filter
	msgStr := logRecord.Msg
	if msgFilter, hasMsgFilter := lf.AttributeFilters["msg"]; hasMsgFilter {
		if strings.Contains(msgStr, msgFilter) && lf.EnableColor {
			msgStr = ColorYellow + msgStr + ColorReset
		}
	}

	output := fmt.Sprintf("%s [%s] %s", timeStr, levelStr, msgStr)

	// Add attributes if any
	if len(logRecord.Attrs) > 0 {
		// Get sorted keys for consistent output
		var keys []string
		for k := range logRecord.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var attrPairs []string
		for _, k := range keys {
			v := logRecord.Attrs[k]
			attrStr := fmt.Sprintf("%s=%v", k, v)

			// Highlight filtered attributes
			if filterValue, isFiltered := lf.AttributeFilters[k]; isFiltered && fmt.Sprintf("%v", v) == filterValue {
				if lf.EnableColor {
					attrStr = ColorYellow + attrStr + ColorGray
				}
			} else if lf.EnableColor {
				attrStr = ColorGray + attrStr
			}

			attrPairs = append(attrPairs, attrStr)
		}
		if len(attrPairs) > 0 {
			if lf.EnableColor {
				output += " " + strings.Join(attrPairs, " ") + ColorReset
			} else {
				output += " " + strings.Join(attrPairs, " ")
			}
		}
	}

	fmt.Println(output)
	return nil
}

// supportsColor checks if the current terminal/environment supports ANSI colors.
// This is best effort. There is no reliable way to determine if a terminal supports
// color. Set NO_COLOR=1 to disable color if your terminal doesn't support it, but this
// function returns true.
func supportsColor() bool {
	// Check NO_COLOR environment variable (universal standard)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check FORCE_COLOR environment variable
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	// On Windows, check for modern terminal support
	if runtime.GOOS == "windows" {
		// Check for Windows Terminal
		if os.Getenv("WT_SESSION") != "" {
			return true
		}
		// Check for ConEmu
		if os.Getenv("ConEmuANSI") == "ON" {
			return true
		}
		// Check for other modern terminals
		term := os.Getenv("TERM")
		if strings.Contains(term, "color") || term == "xterm-256color" || term == "screen-256color" {
			return true
		}
		// Modern PowerShell and cmd.exe with VT processing
		if os.Getenv("TERM_PROGRAM") != "" {
			return true
		}
		// Default to false for older Windows cmd.exe
		return false
	}

	// On Unix-like systems, check TERM
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}

	return true
}

func NewLogFormatter(minLevel string, attributeFilters map[string]string, color bool) *LogFormatter {
	var enableColor bool
	if color && supportsColor() {
		enableColor = true
	}

	return &LogFormatter{
		MinLevel:         minLevel,
		AttributeFilters: attributeFilters,
		EnableColor:      enableColor,
	}
}
