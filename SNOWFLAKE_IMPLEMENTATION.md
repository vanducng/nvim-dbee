# Snowflake Adapter Implementation

This document provides details on the Snowflake adapter implementation for nvim-dbee, including setup, configuration, and testing.

## Overview

The Snowflake adapter provides comprehensive support for Snowflake databases with multiple authentication methods:
- **Password Authentication**: Standard username/password authentication
- **Keypair Authentication**: RSA private key authentication with JWT tokens
- **MFA/SSO Authentication**: Multi-factor authentication via external browser

## Authentication Methods

### 1. Password Authentication (Default)

```lua
{
  name = "Snowflake DB",
  type = "snowflake",
  url = "snowflake://username:password@account.snowflakecomputing.com/database/schema?warehouse=my_warehouse"
}
```

### 2. Keypair Authentication

```lua
{
  name = "Snowflake Keypair",
  type = "snowflake", 
  url = "snowflake://username@account.snowflakecomputing.com/database/schema?authenticator=snowflake_jwt&privateKeyPath=~/.ssh/snowflake_rsa_key.pem&warehouse=my_warehouse"
}
```

**Optional parameters for keypair auth:**
- `privateKeyPassphrase`: Passphrase for encrypted private keys

### 3. MFA/SSO Authentication

```lua
{
  name = "Snowflake SSO",
  type = "snowflake",
  url = "snowflake://username@account.snowflakecomputing.com/database/schema?authenticator=externalbrowser&warehouse=my_warehouse"
}
```

## Connection URL Format

```
snowflake://[user[:password]@]account[/database[/schema]][?param1=value1&paramN=valueN]
```

**Required Components:**
- `user`: Snowflake username
- `account`: Snowflake account identifier (e.g., `myorg-myaccount` or `myaccount.snowflakecomputing.com`)

**Optional Components:**
- `password`: User password (only for password auth)
- `database`: Default database name
- `schema`: Default schema name

**Supported Query Parameters:**
- `authenticator`: Authentication method (`snowflake_jwt`, `externalbrowser`)
- `warehouse`: Snowflake warehouse name
- `privateKeyPath`: Path to RSA private key file (for keypair auth)
- `privateKeyPassphrase`: Passphrase for encrypted private key
- `role`: Snowflake role name
- `timeout`: Connection timeout in seconds

## Environment Variable Support

Use template functions for secure credential management:

```lua
{
  name = "Snowflake Secure",
  type = "snowflake",
  url = "snowflake://{{ env \"SNOWFLAKE_USER\" }}:{{ env \"SNOWFLAKE_PASS\" }}@{{ env \"SNOWFLAKE_ACCOUNT\" }}/{{ env \"SNOWFLAKE_DB\" }}/{{ env \"SNOWFLAKE_SCHEMA\" }}?warehouse={{ env \"SNOWFLAKE_WAREHOUSE\" }}"
}
```

## Private Key Setup for Keypair Authentication

### 1. Generate RSA Key Pair

```bash
# Generate private key
openssl genrsa 2048 | openssl pkcs8 -topk8 -inform PEM -out snowflake_rsa_key.p8 -nocrypt

# Generate public key
openssl rsa -in snowflake_rsa_key.p8 -pubout -out snowflake_rsa_key.pub
```

### 2. Register Public Key in Snowflake

```sql
-- Get the public key (remove header/footer and line breaks)
-- Then register it with your user
ALTER USER myusername SET RSA_PUBLIC_KEY='MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...';
```

### 3. Configure nvim-dbee

```lua
{
  name = "Snowflake Keypair",
  type = "snowflake",
  url = "snowflake://myusername@myaccount.snowflakecomputing.com/mydatabase/myschema?authenticator=snowflake_jwt&privateKeyPath=~/.ssh/snowflake_rsa_key.p8&warehouse=mywarehouse"
}
```

## Features

### Database Operations
- **Structure browsing**: Uses `SHOW TERSE OBJECTS` to avoid warehouse wake-up costs
- **Column information**: Uses `DESC TABLE TYPE = COLUMNS` for efficient metadata retrieval
- **Database switching**: Supports switching between databases
- **Query execution**: Full query support with result streaming

### Cost Optimization
- Uses SHOW statements instead of SELECT queries to avoid starting warehouses
- Efficient metadata queries that don't consume compute credits
- Supports warehouse-specific connections

### Security Features
- Support for encrypted private keys with passphrase
- Template support for environment variables
- No hardcoded credentials in configuration

## Implementation Details

### File Structure
- `snowflake.go`: Adapter implementation with connection logic
- `snowflake_driver.go`: Driver implementation with database operations
- `snowflake_test.go`: Unit tests for authentication methods

### Key Components

1. **URL Parsing**: Intelligent parsing based on authentication method
2. **Private Key Loading**: Support for PKCS1 and PKCS8 formats, encrypted and unencrypted
3. **Connection Management**: Proper resource cleanup and error handling
4. **Query Optimization**: Warehouse-friendly queries for metadata operations

## Testing

### Unit Tests
```bash
# Run Snowflake-specific tests
go test ./adapters -v -run TestSnowflake

# Run all adapter tests
go test ./adapters -v
```

### Manual Testing

1. **Build the plugin**:
   ```bash
   cd dbee && go build
   ```

2. **Configure connection** in your NeoVim setup

3. **Test operations**:
   - Browse database structure
   - View table columns
   - Execute queries
   - Switch databases

## Test Plan

### Authentication Testing
- [x] Password authentication URL parsing
- [x] Keypair authentication URL parsing  
- [x] MFA authentication URL parsing
- [x] Private key loading (PKCS1/PKCS8, encrypted/unencrypted)
- [ ] Connection establishment with real Snowflake instance
- [ ] Token refresh for long-running sessions

### Functionality Testing
- [x] Structure retrieval with SHOW OBJECTS
- [x] Column metadata with DESC TABLE
- [x] Database listing and switching
- [x] Helper query generation
- [ ] Query execution and result handling
- [ ] Error handling and connection recovery

### Integration Testing
- [ ] NeoVim plugin integration
- [ ] Connection persistence across sessions
- [ ] Multiple concurrent connections
- [ ] Large result set handling

### Security Testing
- [ ] Private key file permissions validation
- [ ] Credential template expansion
- [ ] SSL/TLS connection verification
- [ ] Token security and expiration

## Troubleshooting

### Common Issues

1. **Private key format errors**:
   - Ensure key is in PKCS1 or PKCS8 format
   - Check file permissions (should be 600)
   - Verify key encryption/passphrase

2. **Account identifier issues**:
   - Use full account identifier (org-account format)
   - Verify region if using legacy format

3. **Warehouse permissions**:
   - Ensure user has access to specified warehouse
   - Check warehouse auto-suspend settings

4. **Network connectivity**:
   - Verify firewall settings for Snowflake endpoints
   - Check proxy configurations if applicable

### Debug Tips

1. Enable Go driver logging for detailed connection info
2. Use `SHOW PARAMETERS` to verify connection settings
3. Test connection with official Snowflake clients first
4. Check Snowflake query history for actual queries being executed

## Performance Considerations

- Use `SHOW` statements for metadata to avoid warehouse costs
- Implement connection pooling for multiple queries
- Consider warehouse auto-suspend settings
- Use result streaming for large datasets

## Future Enhancements

- [ ] Connection pooling support
- [ ] Query caching for metadata operations
- [ ] Support for Snowflake stages and file operations
- [ ] Integration with Snowflake Python connector features
- [ ] Advanced query profiling and optimization hints