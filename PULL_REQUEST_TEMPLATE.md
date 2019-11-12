## Issue

<!-- link to the issue number or description of the issue -->

## Test

### Unit Test Coverage

Before:

```
<!-- copy/paste 'go test -cover' result in the directory you made change -->
```

After:

```
<!-- copy/paste 'go test -cover' result in the directory you made change -->
```

### Test/Run Logs

<!-- links to the test/run log, or copy&paste part of the log if it is too long -->
<!-- or you may just create a [gist](https://gist.github.com/) and link the gist here -->

## Operational Checklist

1. **Does this PR introduce backward-incompatible changes to the on-disk data structure and/or the over-the-wire protocol?**. (If no, skip to question 8.)

    **YES|NO**

2. **Describe the migration plan.**. For each flag epoch, describe what changes take place at the flag epoch, the anticipated interactions between upgraded/non-upgraded nodes, and any special operational considerations for the migration.

3. **Describe how the plan was tested.**

4. **How much minimum baking period after the last flag epoch should we allow on Pangaea before promotion onto mainnet?**

5. **What are the planned flag epoch numbers and their ETAs on Pangaea?**

6. **What are the planned flag epoch numbers and their ETAs on mainnet?**

    Note that this must be enough to cover baking period on Pangaea.

7. **What should node operators know about this planned change?**

8. **Does this PR introduce backward-incompatible changes *NOT* related to on-disk data structure and/or over-the-wire protocol?** (If no, continue to question 11.)

    **YES|NO**

9. **Does the existing `node.sh` continue to work with this change?**

10. **What should node operators know about this change?**

11. **Does this PR introduce significant changes to the operational requirements of the node software, such as >20% increase in CPU, memory, and/or disk usage?**

## TODO
