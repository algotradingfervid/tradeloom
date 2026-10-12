package secrets

import (
	"errors"
	"fmt"
	"os"
	"reflect"
)

// Mode is what the app must do at start to get a Store.
type Mode string

const (
	ModeKeyring Mode = "keyring" // use NewKeyring()
	ModeFile    Mode = "file"    // ask the passphrase, then OpenEncFile
	ModeChoose  Mode = "choose"  // no keyring and no file: offer exactly Choices
)

// Choice is one option of the no-daemon flow (FR-ACCT-013).
type Choice string

const (
	ChoiceEncryptedFile Choice = "encrypted_file"
	ChoiceNoStorage     Choice = "no_storage"
)

// Detection is the result of Detect.
type Detection struct {
	Mode             Mode
	KeyringAvailable bool
	FileExists       bool
	ProbeError       error    // why the keyring is unavailable (for the S0-4 log line and Settings)
	Choices          []Choice // non-empty only in ModeChoose; never contains a plaintext option
}

// Detect chooses the backend: an existing secrets.enc wins (the user chose it; switching to the
// keyring is an explicit action), else the keyring if probe succeeds, else the two-choice flow.
func Detect(secretsFile string, probe func() error) Detection {
	d := Detection{}
	if _, err := os.Stat(secretsFile); err == nil {
		d.FileExists = true
	}
	d.ProbeError = probe()
	d.KeyringAvailable = d.ProbeError == nil
	switch {
	case d.FileExists:
		d.Mode = ModeFile
	case d.KeyringAvailable:
		d.Mode = ModeKeyring
	default:
		d.Mode = ModeChoose
		d.Choices = []Choice{ChoiceEncryptedFile, ChoiceNoStorage}
	}
	return d
}

// Migrate copies every key of AllKeys present in from into to, verifies each by reading it back,
// then deletes it from from; when from is an *EncFile its file is removed (FR-ACCT-013 AC3).
// If any copy fails, keys already copied are removed from to and from is left untouched. If every key
// was copied and verified but from could not be cleaned up, the error wraps ErrSourceNotRemoved.
func Migrate(from, to Store) error {
	if reflect.TypeOf(from) == reflect.TypeOf(to) && reflect.TypeOf(from).Comparable() && from == to {
		return errors.New("secrets: migrate source and target are the same store")
	}
	type prior struct {
		key     Key
		value   string
		existed bool
	}
	var touched []prior // every key we may have changed in to, recorded BEFORE its Set
	var copied []Key
	rollback := func(cause error) error {
		errs := []error{cause}
		for _, p := range touched {
			if p.existed {
				errs = append(errs, to.Set(p.key, p.value))
			} else {
				errs = append(errs, to.Delete(p.key))
			}
		}
		return errors.Join(errs...)
	}
	for _, k := range AllKeys {
		v, err := from.Get(k)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return rollback(fmt.Errorf("secrets: migrate read %s from %s: %w", k, from.Backend(), err))
		}
		old, err := to.Get(k)
		switch {
		case err == nil:
			touched = append(touched, prior{k, old, true})
		case errors.Is(err, ErrNotFound):
			touched = append(touched, prior{k, "", false})
		default:
			return rollback(fmt.Errorf("secrets: migrate snapshot %s in %s: %w", k, to.Backend(), err))
		}
		if err := to.Set(k, v); err != nil {
			return rollback(fmt.Errorf("secrets: migrate write %s to %s: %w", k, to.Backend(), err))
		}
		copied = append(copied, k)
		if got, err := to.Get(k); err != nil || got != v {
			return rollback(fmt.Errorf("secrets: migrate verify %s in %s failed: %v", k, to.Backend(), err))
		}
	}
	if f, ok := from.(*EncFile); ok {
		if err := f.Destroy(); err != nil {
			return fmt.Errorf("%w: removing the source file failed: %w", ErrSourceNotRemoved, err)
		}
		return nil
	}
	for _, k := range copied {
		if err := from.Delete(k); err != nil {
			return fmt.Errorf("%w: deleting %s from %s failed: %w", ErrSourceNotRemoved, k, from.Backend(), err)
		}
	}
	return nil
}
