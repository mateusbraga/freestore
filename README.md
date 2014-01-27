# FreeStore

FreeStore is a implementation of a consistent and reconfigurable distributed memory. 

FreeStore provides read/write coherence and before-or-after atomicity. Read/write coherence means that the result of the READ is always the same as the most recent received WRITE. Before-or-after atomicity means that the result of every READ or WRITE occurred either completely before or completely after any other READ or WRITE.

FreeStore is currently under heavy development.

[![Build Status](https://travis-ci.org/mateusbraga/freestore.png?branch=master)](https://travis-ci.org/mateusbraga/freestore)
