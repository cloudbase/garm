# OpenStack external provider for garm

This is an example external provider, written for OpenStack. It is a simple bash script that implements the external provider interface, in order to supply ```garm``` with compute instances. This is just an example, complete with a sample config file.

Not all functions are implemented, just the bare minimum to get it to work with the current feature set of ```garm```. It is not meant for production, as it needs a lot more error checking, retries, and potentially more flexibility to be of any use in a real environment.