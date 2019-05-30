## testplan for consensus

### incorrect data

- wrong signature
- construct incorrect consensus messages

### timeout

- leader timeout in normal phase
- new leader timeout in viewchange phase
- multiple new leaders timeout in a row (m < n/3)
- two newview messages arrive at similar time
- a single node stucked at viewchange mode. e.g. new node stucked in view change while others move on

### malicious

- leader broadcasts different blocks in announce phase
- leader send different subsets of prepared signature in prepare phase
- new leader doesn't send prepared message from previous view
- new leader 't send inconsistent announce message from previous view
