# Developer Loop Example

This full plugin has a buildable executable runtime and a desktop contribution. After installing
dependencies, exercise the complete local loop with:

```sh
npm install
wuu plugin build .
wuu plugin test .
wuu plugin dev --watch=false .
```

`plugin test` executes the runtime initialization contract. `plugin dev` builds and validates the
package before atomically publishing an isolated development generation. A failed refresh leaves
the previous development generation intact. Live app-server refresh is not claimed by this
example; restart or use the shell's supported refresh path to consume the published generation.
