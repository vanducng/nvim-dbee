package adapters

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kndndrj/nvim-dbee/dbee/core"
)

func init() {
	register(&Snowflake{}, "snowflake", "sf")
}

type Snowflake struct{}

var _ core.Adapter = (*Snowflake)(nil)

// Connect creates a new Snowflake driver with support for multiple authentication methods
func (s *Snowflake) Connect(urlstr string) (core.Driver, error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection URL: %w", err)
	}

	params := u.Query()
	
	// Build DSN based on authentication method
	authMethod := params.Get("authenticator")
	dsn := ""
	
	// Create a copy of params to preserve original values
	dsnParams := make(url.Values)
	for k, v := range params {
		dsnParams[k] = v
	}
	
	switch authMethod {
	case "snowflake_jwt":
		// Keypair authentication
		dsn = s.buildKeypairDSN(u, dsnParams)
	case "externalbrowser":
		// MFA/SSO authentication
		dsn = s.buildMFADSN(u, dsnParams)
	default:
		// Default password authentication
		dsn = s.buildPasswordDSN(u, dsnParams)
	}

	driver, err := newSnowflakeDriver(dsn, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver: %w", err)
	}

	return driver, nil
}

// buildPasswordDSN builds a DSN for password authentication
func (s *Snowflake) buildPasswordDSN(u *url.URL, params url.Values) string {
	user := u.User.Username()
	pass, _ := u.User.Password()
	account := u.Host
	
	// Extract database/schema from path
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")
	
	dsn := fmt.Sprintf("%s:%s@%s", user, pass, account)
	
	if len(parts) > 0 && parts[0] != "" {
		dsn += "/" + parts[0] // database
		if len(parts) > 1 && parts[1] != "" {
			dsn += "/" + parts[1] // schema
		}
	}
	
	// Add query parameters
	if warehouse := params.Get("warehouse"); warehouse != "" {
		params.Del("warehouse")
		dsn += "?warehouse=" + warehouse
	}
	
	// Add remaining parameters
	if len(params) > 0 {
		if strings.Contains(dsn, "?") {
			dsn += "&" + params.Encode()
		} else {
			dsn += "?" + params.Encode()
		}
	}
	
	return dsn
}

// buildKeypairDSN builds a DSN for keypair authentication
func (s *Snowflake) buildKeypairDSN(u *url.URL, params url.Values) string {
	user := u.User.Username()
	account := u.Host
	
	// Extract database/schema from path
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")
	
	// For keypair auth, we don't include password in DSN
	dsn := fmt.Sprintf("%s@%s", user, account)
	
	if len(parts) > 0 && parts[0] != "" {
		dsn += "/" + parts[0] // database
		if len(parts) > 1 && parts[1] != "" {
			dsn += "/" + parts[1] // schema
		}
	}
	
	// Ensure authenticator is set
	params.Set("authenticator", "snowflake_jwt")
	
	// Remove privateKey from params as it's handled separately in the driver
	params.Del("privateKey")
	params.Del("privateKeyPath")
	params.Del("privateKeyPassphrase")
	
	// Add query parameters
	if strings.Contains(dsn, "?") {
		dsn += "&" + params.Encode()
	} else {
		dsn += "?" + params.Encode()
	}
	
	return dsn
}

// buildMFADSN builds a DSN for MFA/SSO authentication
func (s *Snowflake) buildMFADSN(u *url.URL, params url.Values) string {
	user := u.User.Username()
	account := u.Host
	
	// Extract database/schema from path
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")
	
	// For MFA auth, we don't include password in DSN
	dsn := fmt.Sprintf("%s@%s", user, account)
	
	if len(parts) > 0 && parts[0] != "" {
		dsn += "/" + parts[0] // database
		if len(parts) > 1 && parts[1] != "" {
			dsn += "/" + parts[1] // schema
		}
	}
	
	// Ensure authenticator is set
	params.Set("authenticator", "externalbrowser")
	
	// Add query parameters
	if strings.Contains(dsn, "?") {
		dsn += "&" + params.Encode()
	} else {
		dsn += "?" + params.Encode()
	}
	
	return dsn
}

// GetHelpers returns Snowflake-specific SQL helpers
func (s *Snowflake) GetHelpers(opts *core.TableOptions) map[string]string {
	baseSchema := "DATABASE()"
	schema := baseSchema
	if opts != nil && opts.Schema != "" {
		schema = formatSnowflakeString(opts.Schema)
	}

	table := "t.table_name"
	if opts != nil && opts.Table != "" {
		table = formatSnowflakeString(opts.Table)
	}

	helpers := map[string]string{
		"list": fmt.Sprintf(`
SELECT 
    t.table_catalog,
    t.table_schema,
    t.table_name,
    t.table_type
FROM information_schema.tables t
WHERE t.table_schema = %s
ORDER BY t.table_schema, t.table_name`, schema),

		"columns": fmt.Sprintf(`
SELECT 
    c.column_name,
    c.data_type,
    c.is_nullable,
    c.column_default,
    c.character_maximum_length,
    c.numeric_precision,
    c.numeric_scale
FROM information_schema.columns c
WHERE c.table_schema = %s
  AND c.table_name = %s
ORDER BY c.ordinal_position`, schema, table),

		"indexes": `-- Snowflake doesn't have traditional indexes`,
		
		"constraints": fmt.Sprintf(`
SELECT 
    tc.constraint_name,
    tc.constraint_type,
    kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
    ON tc.constraint_name = kcu.constraint_name
    AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = %s
  AND tc.table_name = %s
ORDER BY tc.constraint_type, kcu.ordinal_position`, schema, table),

		"primary-keys": fmt.Sprintf(`
SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
    ON tc.constraint_name = kcu.constraint_name
    AND tc.table_schema = kcu.table_schema
WHERE tc.table_schema = %s
  AND tc.table_name = %s
  AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`, schema, table),

		"foreign-keys": fmt.Sprintf(`
SELECT 
    kcu.column_name,
    ccu.table_schema AS referenced_schema,
    ccu.table_name AS referenced_table,
    ccu.column_name AS referenced_column
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
    ON tc.constraint_name = kcu.constraint_name
    AND tc.table_schema = kcu.table_schema
JOIN information_schema.constraint_column_usage ccu
    ON ccu.constraint_name = tc.constraint_name
    AND ccu.table_schema = tc.table_schema
WHERE tc.table_schema = %s
  AND tc.table_name = %s
  AND tc.constraint_type = 'FOREIGN KEY'`, schema, table),
	}

	// Only add show-columns if we have table information
	if opts != nil && opts.Schema != "" && opts.Table != "" {
		helpers["show-columns"] = fmt.Sprintf(`DESC TABLE %s.%s TYPE = COLUMNS`, opts.Schema, opts.Table)
	}

	return helpers
}

func formatSnowflakeString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}