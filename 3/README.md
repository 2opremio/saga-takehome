# Code Design Challenge

First of all, I am completely new to Cosmos and IBC. After a whole lot of reading during a couple of hours 
(and learning many concepts which I haven't interiorized yet) here is my understanding of the problem:

## Context

### IBC: Inter-Blockchain Communication

Cosmos blockchains use IBC (Inter-Blockchain Communication), which is an interoperability
communication system between blockchains (I read https://medium.com/the-interchain-foundation/eli5-what-is-ibc-def44d7b5b4c).

Chains can send each other packets (messages) that get delivered by relayers.

One flavor of IBC, Interchain Accounts (ICS-27) (I read https://ida.interchain.io/academy/3-ibc/8-ica.html),
lets Chain A control an account on Chain B, which allows submitting governance or upgrade commands.

### How upgrades normally happen

Every Cosmos SDK chain seems to have a built-in `x/upgrade` ( https://docs.cosmos.network/main/build/modules/upgrade ) module that:

* Lets you store an _upgrade plan_ on-chain: name, when to happen, upgrade info.

* At the right time, validators halt, install the new binary, and restart.

Only the chain’s governance system `x/gov` ( https://docs.cosmos.network/main/build/modules/gov ) can schedule an upgrade.


## My proposal

Use Interchain Accounts (ICS-27) so the Primary cahin controls a dedicated account on the Secondary chain.

A governance action on Secondary delegates the `x/upgrade` authority to that ICA account in the Primary chain in order to make it possible. 

I would elaborate on how to make this happen, but I lack the expertise at this point.

The design seems to be small since it doesn't require a custom IBC application. 