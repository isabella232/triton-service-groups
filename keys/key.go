package keys

import (
	"context"
	"time"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

var (
	ErrExists      = errors.New("can't check existence without id or name")
	ErrNoAccountID = errors.New("missing account identifer for save")
	ErrMissingID   = errors.New("missing identifer for save")
)

// Key represents the data associated with an tsg_keys row.
type Key struct {
	ID          string
	Name        string
	Fingerprint string
	Material    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AccountID   string
	Archived    bool

	store *Store
}

// New constructs a new Key with the Store for backend persistence.
func New(store *Store) *Key {
	return &Key{
		store: store,
	}
}

// Insert inserts a new key into the tsg_keys table.
func (k *Key) Insert(ctx context.Context) error {
	if k.AccountID == "" {
		return ErrNoAccountID
	}

	query := `
INSERT INTO tsg_keys (name, fingerprint, material, account_id, archived, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW());
`

	pool := k.store.pool

	tx, err := pool.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() // nolint: errcheck

	_, err = pool.ExecEx(ctx, query, nil,
		k.Name,
		k.Fingerprint,
		k.Material,
		k.AccountID,
		k.Archived,
	)
	if err != nil {
		return errors.Wrap(err, "failed to insert key")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	key, err := k.store.FindByName(ctx, k.Name, k.AccountID)
	if err != nil {
		return errors.Wrap(err, "failed to find key after insert")
	}

	k.ID = key.ID
	k.CreatedAt = key.CreatedAt
	k.UpdatedAt = key.UpdatedAt

	return nil
}

// Save saves an keys.Key object and it's field values.
func (k *Key) Save(ctx context.Context) error {
	if k.ID == "" {
		return ErrMissingID
	}

	query := `
UPDATE tsg_keys SET (name, fingerprint, material, archived, updated_at) = ($2, $3, $4, $5, $6)
WHERE id = $1;
`
	updatedAt := time.Now()

	pool := k.store.pool

	tx, err := pool.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() // nolint: errcheck

	_, err = pool.ExecEx(ctx, query, nil,
		k.ID,
		k.Name,
		k.Fingerprint,
		k.Material,
		k.Archived,
		updatedAt,
	)
	if err != nil {
		return errors.Wrap(err, "failed to update key")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	k.UpdatedAt = updatedAt

	return nil
}

// Exists returns a boolean and error. True if the row exists, false if it
// doesn't, error if there was an error executing the query.
func (k *Key) Exists(ctx context.Context) (bool, error) {
	if k.Name == "" && k.ID == "" {
		return false, ErrExists
	}

	var count int

	query := `
SELECT 1 FROM tsg_keys
WHERE (id = $1 OR name = $2) AND archived = false;
`
	// NOTE(justinwr): seriously...
	keyID := "00000000-0000-0000-0000-000000000000"
	if k.ID != "" {
		keyID = k.ID
	}

	err := k.store.pool.QueryRowEx(ctx, query, nil,
		keyID,
		k.Name,
	).Scan(&count)
	switch err {
	case nil:
		return true, nil
	case pgx.ErrNoRows:
		return false, nil
	default:
		return false, errors.Wrap(err, "failed to check key existence")
	}
}
