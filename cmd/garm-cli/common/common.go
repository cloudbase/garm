package common

import (
	"errors"

	"github.com/manifoldco/promptui"
	"github.com/nbutton23/zxcvbn-go"
)

func PromptPassword(label string) (string, error) {
	if label == "" {
		label = "Password"
	}
	validate := func(input string) error {
		passwordStenght := zxcvbn.PasswordStrength(input, nil)
		if passwordStenght.Score < 4 {
			return errors.New("password is too weak")
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

func PromptString(label string) (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New("empty input not allowed")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    label,
		Validate: validate,
	}
	result, err := prompt.Run()

	if err != nil {
		return "", err
	}
	return result, nil
}
