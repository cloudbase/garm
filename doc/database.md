# Database configuration

Garm currently supports two database backends:

  * SQLite3
  * MySQL

You can choose either one of these. For most cases, ```SQLite3``` should do, but feel free to go with MySQL if you wish.

```toml
[database]
  # Turn on/off debugging for database queries.
  debug = false
  # Database backend to use. Currently supported backends are:
  #   * sqlite3
  #   * mysql
  backend = "sqlite3"
  # the passphrase option is a temporary measure by which we encrypt the webhook
  # secret that gets saved to the database, using AES256. In the future, secrets
  # will be saved to something like Barbican or Vault, eliminating the need for
  # this.
  passphrase = "n<$n&P#L*TWqOh95_bN5J1r4mhxY7R84HZ%pvM#1vxJ<7~q%YVsCwU@Z60;7~Djo"
  [database.mysql]
    # If MySQL is used, these are the credentials and connection information used
    # to connect to the server instance.
    # database username
    username = ""
    # Database password
    password = ""
    # hostname to connect to
    hostname = ""
    # database name
    database = ""
  [database.sqlite3]
    # Path on disk to the sqlite3 database file.
    db_file = "/home/runner/file.db"
```
