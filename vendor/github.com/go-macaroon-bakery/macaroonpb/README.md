# Macaroon ID Protocol Buffers

This module defines the serialization format of macaroon identifiers for
macaroons created by the macaroon-bakery. For the most part this encoding
is considered an internal implementation detail of the macaroon-bakery
and external applications should not rely on any of the details of this
encoding being maintained between different bakery versions.

This is broken out into a separate module as the protobuf implementation
works in such a way that one cannot have multiple definitions of a
message in any particular application's dependency tree. This module
therefore provides a common definition for use by multiple versions of
the macaroon-bakery to facilitate easier migration in client applications.
