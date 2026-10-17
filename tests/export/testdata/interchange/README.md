The two library identity JSON fixtures are toolkit v0.10.0 full-JSON exports with a hand-edited `ScalarValues::Real` library stub id; the unmatched case also changes its qualified name.

The `satisfy_by` fixture is a toolkit v0.10.0 full-JSON export of the adjacent source model.

The `state_entry` fixture is a toolkit v0.10.0 full-JSON export of the adjacent source model; its `entry; then idle;` succession sequences from the nameless entry action.

The `states` fixtures are toolkit v0.10.10 compact-JSON and full-JSON exports of the adjacent source model; `state def T` owns the referencing `do x;` subaction so that `state def S` owns one subaction per kind.

The `p10`, `state_entry`, `states` and `transitions` full-JSON exports are committed gzip-compressed (`gzip -9 -n`, so recompressing the same export gives the same bytes); the tests read them through `toolkitFixture`, which decompresses a `.json.gz` when the `.json` is absent. `gunzip -k` restores the plain export.
