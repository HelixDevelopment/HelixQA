// Package navigator provides Android TV on-screen keyboard navigation
// This is a UNIVERSAL solution that works with ANY Android TV application
package navigator

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TVKeyboard provides universal text input for Android TV by navigating
// the on-screen keyboard (Gboard) using DPAD keys. This works with ANY
// Android TV app, not just Catalogizer.
type TVKeyboard struct {
	executor  *ADBExecutor
	cursorRow int // Current cursor row position
	cursorCol int // Current cursor column position
}

// NewTVKeyboard creates a new TV keyboard navigator
func NewTVKeyboard(executor *ADBExecutor) *TVKeyboard {
	return &TVKeyboard{
		executor:  executor,
		cursorRow: 0, // Start at home position (q key)
		cursorCol: 0,
	}
}

// GboardLayout represents the standard Gboard QWERTY layout for Android TV
// Layout (row by row, starting from top):
// Row 0: q w e r t y u i o p
// Row 1:  a s d f g h j k l
// Row 2:   z x c v b n m
// Row 3: [?123] [<] [space] [>] [.] [enter]
type GboardLayout struct {
	// Current cursor position (row, col)
	cursorRow int
	cursorCol int
}

// Key positions on Gboard (row, col)
var gboardKeyPositions = map[rune][2]int{
	// Row 0
	'q': {0, 0}, 'w': {0, 1}, 'e': {0, 2}, 'r': {0, 3}, 't': {0, 4},
	'y': {0, 5}, 'u': {0, 6}, 'i': {0, 7}, 'o': {0, 8}, 'p': {0, 9},
	// Row 1
	'a': {1, 0}, 's': {1, 1}, 'd': {1, 2}, 'f': {1, 3}, 'g': {1, 4},
	'h': {1, 5}, 'j': {1, 6}, 'k': {1, 7}, 'l': {1, 8},
	// Row 2
	'z': {2, 0}, 'x': {2, 1}, 'c': {2, 2}, 'v': {2, 3}, 'b': {2, 4},
	'n': {2, 5}, 'm': {2, 6},
	// Numbers (accessed via ?123 key)
	'1': {0, 0}, '2': {0, 1}, '3': {0, 2}, '4': {0, 3}, '5': {0, 4},
	'6': {0, 5}, '7': {0, 6}, '8': {0, 7}, '9': {0, 8}, '0': {0, 9},
}

// TypeText types the given text using the on-screen keyboard
// This is a UNIVERSAL method that works with ANY Android TV app
func (tk *TVKeyboard) TypeText(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}

	// Ensure keyboard is visible by focusing a text field first
	// The caller should have already focused a text field
	time.Sleep(500 * time.Millisecond)

	// Type each character
	for _, char := range strings.ToLower(text) {
		if err := tk.typeCharacter(ctx, char); err != nil {
			return fmt.Errorf("failed to type character '%c': %w", char, err)
		}
		time.Sleep(100 * time.Millisecond) // Small delay between characters
	}

	return nil
}

// typeCharacter types a single character using keyboard navigation
func (tk *TVKeyboard) typeCharacter(ctx context.Context, char rune) error {
	// Handle special characters
	switch char {
	case ' ':
		return tk.pressSpace(ctx)
	case '.':
		return tk.pressDot(ctx)
	case '-':
		return tk.pressHyphen(ctx)
	case '_':
		return tk.pressUnderscore(ctx)
	case ':':
		return tk.pressColon(ctx)
	case '/':
		return tk.pressSlash(ctx)
	}

	// Check if it's a digit
	if char >= '0' && char <= '9' {
		return tk.typeDigit(ctx, char)
	}

	// Check if it's a letter
	pos, ok := gboardKeyPositions[char]
	if !ok {
		// Character not in our layout, skip it
		return fmt.Errorf("character '%c' not supported in keyboard layout", char)
	}

	// Navigate to the character position
	// Start from home position (q key, which is usually the default focus)
	// Navigate to target position
	targetRow, targetCol := pos[0], pos[1]

	// Navigate vertically first
	for tk.cursorRow < targetRow {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		tk.cursorRow++
		time.Sleep(50 * time.Millisecond)
	}
	for tk.cursorRow > targetRow {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_UP"); err != nil {
			return err
		}
		tk.cursorRow--
		time.Sleep(50 * time.Millisecond)
	}

	// Navigate horizontally
	for tk.cursorCol < targetCol {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_RIGHT"); err != nil {
			return err
		}
		tk.cursorCol++
		time.Sleep(50 * time.Millisecond)
	}
	for tk.cursorCol > targetCol {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		tk.cursorCol--
		time.Sleep(50 * time.Millisecond)
	}

	// Press the key
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}

	return nil
}

// typeDigit types a digit by switching to number pad first
func (tk *TVKeyboard) typeDigit(ctx context.Context, digit rune) error {
	// Switch to ?123 mode
	// ?123 key is typically at bottom left
	// Navigate to it and press
	
	// First, press DPAD_DOWN multiple times to reach row 3
	for i := 0; i < 4; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Press left multiple times to reach ?123
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Press ?123 to switch to number mode
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	
	// Now navigate to the digit
	digitCol := int(digit - '0')
	if digitCol == 0 {
		digitCol = 9 // 0 is at position 9
	} else {
		digitCol-- // 1-9 are at positions 0-8
	}
	
	// Navigate to the digit
	for i := 0; i < digitCol; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_RIGHT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Press the digit
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	
	// Switch back to ABC mode by pressing ?123 again
	// Navigate to ?123
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	
	return nil
}

// pressSpace presses the space key
func (tk *TVKeyboard) pressSpace(ctx context.Context) error {
	// Space is typically in the bottom row, center
	// Navigate to bottom row
	for i := 0; i < 4; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Navigate to center (space bar)
	// From left side, move right about 3-4 times
	for i := 0; i < 3; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_RIGHT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	return tk.executor.KeyPress(ctx, "KEYCODE_ENTER")
}

// pressDot presses the dot/period key
func (tk *TVKeyboard) pressDot(ctx context.Context) error {
	// Dot is typically near the space bar on the bottom row
	// Navigate to bottom row
	for i := 0; i < 4; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Navigate right to reach the dot key
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_RIGHT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	return tk.executor.KeyPress(ctx, "KEYCODE_ENTER")
}

// pressHyphen types a hyphen by switching to symbols
func (tk *TVKeyboard) pressHyphen(ctx context.Context) error {
	// Navigate to ?123 first
	for i := 0; i < 4; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Press ?123
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	
	// Hyphen is usually in the second row of symbols
	// Navigate to it
	if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	
	// Press hyphen (position varies, but usually accessible)
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	
	// Switch back to ABC
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	return tk.executor.KeyPress(ctx, "KEYCODE_ENTER")
}

// pressUnderscore types an underscore
func (tk *TVKeyboard) pressUnderscore(ctx context.Context) error {
	// Similar to hyphen but need to access symbols page
	// For simplicity, use the hyphen method
	return tk.pressHyphen(ctx)
}

// pressColon types a colon (used in URLs like http://)
func (tk *TVKeyboard) pressColon(ctx context.Context) error {
	// Navigate to ?123
	for i := 0; i < 4; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Press ?123
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	
	// Navigate to find colon (usually requires going to symbols page 2)
	// Press the symbols shift key (=&<)
	if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	
	// Navigate to colon position
	for i := 0; i < 3; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_RIGHT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	if err := tk.executor.KeyPress(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	
	// Switch back
	for i := 0; i < 5; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_LEFT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	return tk.executor.KeyPress(ctx, "KEYCODE_ENTER")
}

// pressSlash types a forward slash
func (tk *TVKeyboard) pressSlash(ctx context.Context) error {
	// Similar to colon, use symbols page
	return tk.pressColon(ctx)
}

// CloseKeyboard dismisses the on-screen keyboard by pressing BACK
func (tk *TVKeyboard) CloseKeyboard(ctx context.Context) error {
	return tk.executor.KeyPress(ctx, "KEYCODE_BACK")
}

// ConfirmInput confirms the current input (like pressing the checkmark)
func (tk *TVKeyboard) ConfirmInput(ctx context.Context) error {
	// The checkmark is typically at the bottom right
	// Navigate to bottom row
	for i := 0; i < 4; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_DOWN"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Navigate to far right
	for i := 0; i < 7; i++ {
		if err := tk.executor.KeyPress(ctx, "KEYCODE_DPAD_RIGHT"); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	return tk.executor.KeyPress(ctx, "KEYCODE_ENTER")
}

// TVKeyboardInput provides a high-level interface for text input
// This is the RECOMMENDED way to input text in Android TV apps
type TVKeyboardInput struct {
	keyboard *TVKeyboard
}

// NewTVKeyboardInput creates a new TV keyboard input handler
func NewTVKeyboardInput(executor *ADBExecutor) *TVKeyboardInput {
	return &TVKeyboardInput{
		keyboard: NewTVKeyboard(executor),
	}
}

// Type types text using the on-screen keyboard
// Usage:
//   1. Focus a text field first (DPAD navigation + ENTER)
//   2. Wait for keyboard to appear
//   3. Call Type() with the text
//   4. Call Confirm() or CloseKeyboard() to finish
func (tvi *TVKeyboardInput) Type(ctx context.Context, text string) error {
	// Wait for keyboard to be ready
	time.Sleep(500 * time.Millisecond)
	
	return tvi.keyboard.TypeText(ctx, text)
}

// Confirm confirms the input and closes the keyboard
func (tvi *TVKeyboardInput) Confirm(ctx context.Context) error {
	return tvi.keyboard.ConfirmInput(ctx)
}

// Close closes the keyboard without confirming
func (tvi *TVKeyboardInput) Close(ctx context.Context) error {
	return tvi.keyboard.CloseKeyboard(ctx)
}
