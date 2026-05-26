# ADR 0001 — Response shape is a contract

Status: Accepted

The shape of every public response is part of the service contract.
Downstream consumers parse responses strictly, and CI fails when an
endpoint's live response drifts from its versioned contract under
`docs/api/`. Any change to a response payload requires the contract
file to be updated in the same commit.
