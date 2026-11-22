# Tasks

## 1. Contracts and domain

- [ ] 1.1 Decision record 0013. Proto: registry RPCs and `ShadowCompared`. Domain: gates, `ErrPromotionRequired`.

## 2. Registry

- [ ] 2.1 Migration (approvals, shadow comparisons); repository methods in both adapters with contract tests; promote, rollback, approve, gates use cases; gRPC methods; shadow-stats consumer.

## 3. Worker

- [ ] 3.1 Shadow slot, shadow comparison, `ShadowCompared` publish. Verify the shadow scenarios.

## 4. Real run

- [ ] 4.1 Shadow v2 against production v3, approve, promote, roll back; measure agreement and how long rollback takes to reach the worker.
