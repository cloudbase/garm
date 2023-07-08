# Database configuration

GARM currently supports SQLite3. Support for other stores will be added in the future.

```toml
[database]
  # Turn on/off debugging for database queries.
  debug = false
  # Database backend to use. Currently supported backends are:
  #   * sqlite3
  backend = "sqlite3"
  # the passphrase option is a temporary measure by which we encrypt the webhook
  # secret that gets saved to the database, using AES256. In the future, secrets
  # will be saved to something like Barbican or Vault, eliminating the need for
  # this.
  passphrase = "n<$n&P#L*TWqOh95_bN5J1r4mhxY7R84HZ%pvM#1vxJ<7~q%YVsCwU@Z60;7~Djo"
  [database.sqlite3]
    # Path on disk to the sqlite3 database file.
    db_file = "/home/runner/garm.db"
```
