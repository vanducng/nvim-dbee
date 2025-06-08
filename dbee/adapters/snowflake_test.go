package adapters

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnowflake_buildPasswordDSN(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedDSN string
	}{
		{
			name:        "basic password auth",
			inputURL:    "snowflake://user:pass@account.snowflakecomputing.com/database/schema?warehouse=warehouse1",
			expectedDSN: "user:pass@account.snowflakecomputing.com/database/schema?warehouse=warehouse1",
		},
		{
			name:        "without schema",
			inputURL:    "snowflake://user:pass@account.snowflakecomputing.com/database?warehouse=warehouse1",
			expectedDSN: "user:pass@account.snowflakecomputing.com/database?warehouse=warehouse1",
		},
		{
			name:        "without database",
			inputURL:    "snowflake://user:pass@account.snowflakecomputing.com?warehouse=warehouse1",
			expectedDSN: "user:pass@account.snowflakecomputing.com?warehouse=warehouse1",
		},
	}

	s := &Snowflake{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.inputURL)
			assert.NoError(t, err)
			
			params := u.Query()
			result := s.buildPasswordDSN(u, params)
			assert.Equal(t, tt.expectedDSN, result)
		})
	}
}

func TestSnowflake_buildKeypairDSN(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedDSN string
	}{
		{
			name:        "keypair auth",
			inputURL:    "snowflake://user@account.snowflakecomputing.com/database/schema?privateKeyPath=/path/to/key.pem&warehouse=warehouse1",
			expectedDSN: "user@account.snowflakecomputing.com/database/schema?authenticator=snowflake_jwt&privateKeyPath=%2Fpath%2Fto%2Fkey.pem&warehouse=warehouse1",
		},
		{
			name:        "keypair auth without schema",
			inputURL:    "snowflake://user@account.snowflakecomputing.com/database?privateKeyPath=/path/to/key.pem",
			expectedDSN: "user@account.snowflakecomputing.com/database?authenticator=snowflake_jwt&privateKeyPath=%2Fpath%2Fto%2Fkey.pem",
		},
	}

	s := &Snowflake{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.inputURL)
			assert.NoError(t, err)
			
			params := u.Query()
			result := s.buildKeypairDSN(u, params)
			assert.Equal(t, tt.expectedDSN, result)
		})
	}
}

func TestSnowflake_buildMFADSN(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedDSN string
	}{
		{
			name:        "MFA auth",
			inputURL:    "snowflake://user@account.snowflakecomputing.com/database/schema?warehouse=warehouse1",
			expectedDSN: "user@account.snowflakecomputing.com/database/schema?authenticator=externalbrowser&warehouse=warehouse1",
		},
	}

	s := &Snowflake{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.inputURL)
			assert.NoError(t, err)
			
			params := u.Query()
			result := s.buildMFADSN(u, params)
			assert.Equal(t, tt.expectedDSN, result)
		})
	}
}

func TestSnowflake_GetHelpers(t *testing.T) {
	s := &Snowflake{}
	
	helpers := s.GetHelpers(nil)
	
	// Check that essential helpers are present
	assert.Contains(t, helpers, "list")
	assert.Contains(t, helpers, "columns")
	assert.Contains(t, helpers, "constraints")
	assert.Contains(t, helpers, "primary-keys")
	assert.Contains(t, helpers, "foreign-keys")
	
	// show-columns should not be present when opts is nil
	assert.NotContains(t, helpers, "show-columns")
	
	// Check that list query contains expected elements
	assert.Contains(t, helpers["list"], "information_schema.tables")
	assert.Contains(t, helpers["columns"], "information_schema.columns")
}

