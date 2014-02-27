# Freestore

Freestore is a library that implements a consistent and reconfigurable distributed memory. 

Freestore implements a Storage abstraction with READ/WRITE operations. It replicates the data on all servers and uses the idea that two majorities always intersect to provide fault-tolerance. Freestore also implements a reconfiguration mechanism that allows the cluster to continue operating normally during configuration changes.

Freestore provides read/write coherence and before-or-after atomicity. Read/write coherence means that the result of the READ is always the same as the most recent received WRITE. Before-or-after atomicity means that the result of every READ or WRITE occurred either completely before or completely after any other READ or WRITE.


Freestore is currently under heavy development.

[![Build Status](https://travis-ci.org/mateusbraga/freestore.png?branch=master)](https://travis-ci.org/mateusbraga/freestore)
