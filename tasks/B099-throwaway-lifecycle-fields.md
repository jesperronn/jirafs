# B099: exercise more fields in the throwaway lifecycle test

After the summary push on SKOLELOGIN-18655 works, extend the lifecycle test
(pull → edit → diff → push → push --apply → pull) to these fields:

- Fix version (fix_versions)
- Labels
- Acceptkriterie (acceptance criteria custom field; find its customfield id
  via createmeta / editmeta and map it like other custom_fields)

Done when each field round-trips: edited locally, pushed with `push --apply`, and confirmed by a
fresh `pull`.
