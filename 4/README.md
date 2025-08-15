# Code Debug Challenge

What I can see is that a new message called `MsgBrkchain` (and associated CLI command) was half-added:

```
fons@autoamp:~/downloaded/brk-cosmossdk$ git status
On branch master
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   docs/static/openapi.yml
	modified:   proto/brkcosmossdk/brkcosmossdk/tx.proto
	modified:   readme.md
	modified:   x/brkcosmossdk/client/cli/tx.go
	modified:   x/brkcosmossdk/module_simulation.go
	modified:   x/brkcosmossdk/types/codec.go
	modified:   x/brkcosmossdk/types/tx.pb.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	build.sh
	build/
	start.sh
	val2app.toml
	val2config.toml
	val3app.toml
	val3config.toml
	x/brkcosmossdk/client/cli/tx_brkchain.go
	x/brkcosmossdk/keeper/msg_server_brkchain.go
	x/brkcosmossdk/simulation/brkchain.go
	x/brkcosmossdk/types/message_brkchain.go
	x/brkcosmossdk/types/message_brkchain_test.go

no changes added to commit (use "git add" and/or "git commit -a")
```

The semantics of the message aren't clear out of its name (it just contains the address of the originating node).

What I can see is that consensus isn't reached after sending the message through the CLI 
(as prompted by the instructions in the readme), resulting in errors like the following in all the validators.

That is, after running: `dosomething tx brkcosmossdk brkchain --from validator2 --home /tmp/val2 --keyring-backend test --fees 100stake --chain-id dosomething -y` 

I get:

```
8:39PM ERR prevote step: consensus deems this block invalid; prevoting nil err="wrong Block.Header.LastResultsHash.  Expected 75C4AD44666F511E4AB6C0985CF9BD7D31DB352AEEB42EC9102A7327D60592F0, got 1318E3F0DF1944FEC0F59C3890F3B0DDCAA2CAA1762CE7DD8085782D36D5462D" height=3 module=consensus round=5
```

After inspecting the code thoroughly, the problem seems to lie at `func (k msgServer) Brkchain()` ( at `x/brkcosmossdk/keeper/msg_server_brkchain.go`).

In particular, at:

```go
	for i := 0; i < types.NumIterations; i++ {
		mymap[i] = i
	}

	// iterate over the map
	for ky, vl := range mymap {

		store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(MyStoreKey))

		key := []byte(strconv.Itoa(ky))
		if store.Has(key) {
			return nil, fmt.Errorf("key already exists in store")
		}

		value := []byte(strconv.Itoa(vl))
		if len(value) == 0 {
			return nil, fmt.Errorf("value cannot be 0 length")
		}

		store.Set(key, value)
	}
```

Upon commenting that code, the error disappears. The code creates a store and modifies it, but it's not clear
what's the use of storing keys identical to their values up to `NumIterations`.


Regardless, the problem seems to come from the fact that map iterations in Go are not deterministic (https://go.dev/blog/maps#iteration-order ). 
Which will cause different validator nodes to run `store.Set(key, value)` in a different order (resulting in different blocks).


If we get rid of the map and modify that code into the equivalent (and more efficient):

```
	for i := 0; i < types.NumIterations; i++ {
		store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte(MyStoreKey))

		key := []byte(strconv.Itoa(i))
		if store.Has(key) {
			return nil, fmt.Errorf("key already exists in store")
		}

		value := []byte(strconv.Itoa(i))
		if len(value) == 0 {
			return nil, fmt.Errorf("value cannot be 0 length")
		}

		store.Set(key, value)
	}

```

Then the CLI command doesn't cause the chain to halt.





