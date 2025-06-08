package adapters

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/kndndrj/nvim-dbee/dbee/core"
	"github.com/kndndrj/nvim-dbee/dbee/core/builders"
	_ "github.com/snowflakedb/gosnowflake"
	"github.com/snowflakedb/gosnowflake"
)

type snowflakeDriver struct {
	c              *builders.Client
	connectionParams url.Values
}

var (
	_ core.Driver           = (*snowflakeDriver)(nil)
	_ core.DatabaseSwitcher = (*snowflakeDriver)(nil)
)

func newSnowflakeDriver(dsn string, params url.Values) (*snowflakeDriver, error) {
	// Handle keypair authentication if specified
	if params.Get("authenticator") == "snowflake_jwt" {
		privateKeyPath := params.Get("privateKeyPath")
		privateKeyPass := params.Get("privateKeyPassphrase")
		
		if privateKeyPath != "" {
			// Expand ~ to home directory
			if strings.HasPrefix(privateKeyPath, "~") {
				home, err := os.UserHomeDir()
				if err != nil {
					return nil, fmt.Errorf("failed to get home directory: %w", err)
				}
				privateKeyPath = strings.Replace(privateKeyPath, "~", home, 1)
			}
			
			// Load private key
			privateKey, err := loadPrivateKey(privateKeyPath, privateKeyPass)
			if err != nil {
				return nil, fmt.Errorf("failed to load private key: %w", err)
			}
			
			// Parse DSN to get config
			cfg, err := gosnowflake.ParseDSN(dsn)
			if err != nil {
				return nil, fmt.Errorf("failed to parse DSN: %w", err)
			}
			
			// Set private key in config
			cfg.PrivateKey = privateKey
			cfg.Authenticator = gosnowflake.AuthTypeJwt
			
			// Create DSN from config
			dsn, err = gosnowflake.DSN(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create DSN from config: %w", err)
			}
		}
	}
	
	// Open connection
	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	client := builders.NewClient(db)

	return &snowflakeDriver{
		c:              client,
		connectionParams: params,
	}, nil
}

// loadPrivateKey loads an RSA private key from file
func loadPrivateKey(path, passphrase string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	var privateKey *rsa.PrivateKey
	
	if x509.IsEncryptedPEMBlock(block) {
		// Decrypt the key if it's encrypted
		decryptedBytes, err := x509.DecryptPEMBlock(block, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt private key: %w", err)
		}
		
		parsedKey, err := x509.ParsePKCS1PrivateKey(decryptedBytes)
		if err != nil {
			// Try PKCS8 format
			key, err := x509.ParsePKCS8PrivateKey(decryptedBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key: %w", err)
			}
			var ok bool
			privateKey, ok = key.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("private key is not RSA")
			}
		} else {
			privateKey = parsedKey
		}
	} else {
		// Parse unencrypted key
		parsedKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			// Try PKCS8 format
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key: %w", err)
			}
			var ok bool
			privateKey, ok = key.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("private key is not RSA")
			}
		} else {
			privateKey = parsedKey
		}
	}

	return privateKey, nil
}

func (d *snowflakeDriver) Query(ctx context.Context, query string) (core.ResultStream, error) {
	return d.c.Query(ctx, query)
}

func (d *snowflakeDriver) Structure() ([]*core.Structure, error) {
	// Use SHOW OBJECTS to avoid waking warehouse
	query := `SHOW TERSE OBJECTS`
	
	result, err := d.c.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute structure query: %w", err)
	}
	defer result.Close()

	var structures []*core.Structure
	for result.HasNext() {
		row, err := result.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to get next row: %w", err)
		}

		// SHOW TERSE OBJECTS returns: created_on, name, kind, database_name, schema_name
		if len(row) < 5 {
			continue
		}

		name, _ := row[1].(string)
		kind, _ := row[2].(string)
		schemaName, _ := row[4].(string)

		// Skip INFORMATION_SCHEMA objects
		if schemaName == "INFORMATION_SCHEMA" {
			continue
		}

		// Only include tables and views
		if kind == "TABLE" || kind == "VIEW" {
			structures = append(structures, &core.Structure{
				Name:   name,
				Schema: schemaName,
				Type:   core.StructureTypeFromString(kind),
			})
		}
	}

	return structures, nil
}

func (d *snowflakeDriver) Columns(opts *core.TableOptions) ([]*core.Column, error) {
	if opts == nil || opts.Table == "" {
		return nil, fmt.Errorf("table options with table name required")
	}

	// Use DESC TABLE to avoid waking warehouse
	query := fmt.Sprintf("DESC TABLE %s.%s TYPE = COLUMNS", opts.Schema, opts.Table)
	
	result, err := d.c.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute columns query: %w", err)
	}
	defer result.Close()

	var columns []*core.Column
	for result.HasNext() {
		row, err := result.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to get next row: %w", err)
		}

		// DESC TABLE TYPE = COLUMNS returns: name, type, kind, null?, default, primary key, unique key, check, expression, comment, policy name
		if len(row) < 2 {
			continue
		}

		name, _ := row[0].(string)
		dataType, _ := row[1].(string)

		columns = append(columns, &core.Column{
			Name: name,
			Type: dataType,
		})
	}

	return columns, nil
}

func (d *snowflakeDriver) SelectDatabase(name string) error {
	_, err := d.c.Exec(context.Background(), fmt.Sprintf("USE DATABASE %s", name))
	if err != nil {
		return fmt.Errorf("failed to select database: %w", err)
	}
	return nil
}

func (d *snowflakeDriver) ListDatabases() (current string, available []string, err error) {
	// Get current database
	result, err := d.c.Query(context.Background(), "SELECT CURRENT_DATABASE()")
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current database: %w", err)
	}
	defer result.Close()

	if result.HasNext() {
		row, err := result.Next()
		if err == nil && len(row) > 0 {
			if currentDB, ok := row[0].(string); ok {
				current = currentDB
			}
		}
	}

	// List all databases
	result, err = d.c.Query(context.Background(), "SHOW DATABASES")
	if err != nil {
		return current, nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer result.Close()

	for result.HasNext() {
		row, err := result.Next()
		if err != nil {
			return current, available, fmt.Errorf("failed to get next row: %w", err)
		}

		// SHOW DATABASES returns multiple columns, we only need the name (2nd column)
		if len(row) >= 2 {
			if name, ok := row[1].(string); ok {
				available = append(available, name)
			}
		}
	}

	return current, available, nil
}

func (d *snowflakeDriver) Close() {
	d.c.Close()
}