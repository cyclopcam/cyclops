# Modules

In order to solve the icky problem of shipping a binary that uses rare hardware
facilities, I decided to make an dynamically loadable NN module system.
Basically, we'll attempt to `dlopen` a module, and if it works, then we use it.

Hailo was the first thing to require this. I didn't want to make the Cyclops Go
binary dependent on the Hailo runtime library.
