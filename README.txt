Ideias:

sample_server.go

group.go
    - View
        - View's vector clock
    - Partition detection
    - Failure detection
    - Cryptography
group/communication.go
    - view-synchrony: Necessário para primary-backup

---

Requirements:
    Stable storage: Atomic write operation and failure tolerance to
    implement permanent storage.
        Required by: Consensus
    Reliable communication: Like TCP.

---

TODO: 

Usar mais interfaces, por exemplo, register poderia ser uma interface.
View generators também poderia ser uma interface

