# Weekly reports

Weekly reports use the active workspace and a Monday date (`--week YYYY-MM-DD`).
An agent acts for the member bound to its credential, normally the runtime owner.
It cannot choose another member for writes. Other members' drafts stay private.
Owner/admin credentials can update the team's overall progress.

```bash
multica weekly-report list --week 2026-11-09
multica weekly-report get --week 2026-11-09
multica weekly-report get --week 2026-11-09 --member <user-uuid>
multica weekly-report history --limit 20
multica weekly-report team get --week 2026-11-09
```

All commands return JSON. Read the current `revision` before writing; pass it as
`--revision`. New reports start at revision 0. A 409 means another writer changed
the report: read it again, reconcile the requested edits, and save the new revision.
Do not blindly retry the old document or replace fields unrelated to the request.

Personal `save` takes a content document in a file or stdin (`--file -`):

```json
{"tasks":[{"id":"stable-row-id","version":"3.0.0","title":"Connector integration","completed":"Completed adapter tests","spent":1.5,"remaining":0,"note":""}],"plan":"Verify onsite"}
```

`spent` and `remaining` are person-days. Use null for unknown, 0 for zero.
Preserve existing row IDs; omitted IDs on new rows are assigned by the server.

```bash
multica weekly-report save --week 2026-11-09 --revision 0 --file content.json
multica weekly-report submit --week 2026-11-09 --revision <saved-revision>
```

Saving only updates a private draft. Submit explicitly when the user asks to
publish; the saved draft then becomes visible to workspace members. Later draft
edits leave the last published version visible until the next submit.

Team overall progress is a separate document, with free-text progress fields:

```json
{"version_name":"Logits-3.0.0","version_rows":[{"id":"version-row","title":"Skill configuration","type":"Feature","last_progress":"Testing","this_plan":"Finish regression","note":""}],"onsite_rows":[{"id":"site-row","title":"Customer connector","type":"Customization","last_progress":"Adapter ready","this_plan":"Onsite verification","note":""}]}
```

```bash
multica weekly-report team save --week 2026-11-09 --revision 0 --file overview.json
multica weekly-report team generate --week 2026-11-09 --revision <saved-revision>
```

Team generation snapshots the saved overview and every submitted personal report
on the server. It excludes drafts. Subsequent submissions do not alter a generated
report; generate again only when a refreshed report is wanted. Team `get` returns
the live `collection`, the last generated `snapshot`, and the overall `draft` only
to owner/admin credentials. Updating overall progress does not submit anybody's
personal report or automatically replace the generated snapshot.
