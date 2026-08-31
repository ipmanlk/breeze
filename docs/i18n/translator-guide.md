# Translator Guide: Editing XLIFF Files

> **For translators and developers editing translations directly.**
> No code knowledge required. If you can edit XML, you can translate Plume.
> Related: [adding-a-language.md](./adding-a-language.md) (developer setup)

Plume's user-facing strings are stored in **XLIFF 1.2** files (`.xlf`), one
per locale, in `ui/src/i18n/messages/`. This is the industry-standard
translation format, so the same files work whether you edit them by hand or
import them into a professional translation tool (Crowdin, Lokalise,
Transifex, Phrase, memoQ, Trados).

---

## What you edit

You translate **one file per locale**: `ui/src/i18n/messages/<locale>.xlf`.

| Locale | File | Language |
|---|---|---|
| `fr` | `messages/fr.xlf` | French |
| `es` | `messages/es.xlf` | Spanish |
| `en` | (none; `en` is the source language) | English |

You **only fill in `<target>` elements**. Never edit `<source>`; those are
regenerated from the code automatically and your edits would be overwritten.

---

## XLIFF structure

A `.xlf` file looks like this:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<xliff version="1.2">
  <file source-language="en" target-language="fr" datatype="plaintext">
    <body>

      <!-- A simple string -->
      <trans-unit id="save_changes">
        <source>Save changes</source>
        <target>Enregistrer les modifications</target>
      </trans-unit>

      <!-- A string with a placeholder (see below) -->
      <trans-unit id="welcome_name">
        <source>Welcome, <x equiv-text="name"/>!</source>
        <target>Bienvenue, <x equiv-text="name"/> !</target>
      </trans-unit>

    </body>
  </file>
</xliff>
```

Your job: for every `<trans-unit>`, write the translation inside `<target>...</target>`.

---

## Placeholders: the most important rule

Some strings contain dynamic values (a user's name, a count, a project name).
In the `.xlf` file these appear as `<x equiv-text="…"/>` placeholder markers.
They look like this:

```xml
<source>Welcome, <x equiv-text="name"/>!</source>
```

**You must preserve these markers exactly as they appear**, moving them to
the correct position in your translation:

```xml
<!-- ✅ Correct: placeholder moved to the right place for French -->
<target>Bienvenue, <x equiv-text="name"/> !</target>

<!-- ❌ Wrong: placeholder removed -->
<target>Bienvenue !</target>

<!-- ❌ Wrong: placeholder renamed -->
<target>Bienvenue, <x equiv-text="nom"/> !</target>
```

If you see multiple placeholders, keep all of them and put them in the
grammatically correct order for your language:

```xml
<source><x equiv-text="actor"/> mentioned you in <x equiv-text="org"/></source>
<target><x equiv-text="actor"/> vous a mentionné dans <x equiv-text="org"/></target>
```

---

## Plurals

English has two plural forms (`one` / `other`); some languages have more (`few`, `many`,
`two`, `zero`). Plume handles plurals with **one `<trans-unit>` per plural category**, each
with a distinct `id` suffix:

```xml
<!-- "1 comment" -->
<trans-unit id="comments_one">
  <source>{count} comment</source>
  <target>{count} commentaire</target>
</trans-unit>

<!-- "0, 2+ comments" (in English) -->
<trans-unit id="comments_other">
  <source>{count} comments</source>
  <target>{count} commentaires</target>
</trans-unit>
```

Notes:
- The `{count}` placeholder is interpolated at runtime; translate it as a normal placeholder.
- **Different languages group numbers differently.** In French, `0` and `1` both use the `one`
  form; in English, `0` uses `other`. You don't need to decide which form applies to which
  number; the code handles that via [CLDR plural rules](https://unicode.org/cldr/charts/latest/supplemental/language_plural_rules.html).
  Your job is just to translate each form (`_one`, `_other`, `_few`, `_many`, …) that appears
  in the file.
- If your language has a plural category that English doesn't (e.g. Russian `few`/`many`),
  contact a developer; those units need to be added via the [adding-a-language workflow](./adding-a-language.md).

---

## Workflow

1. **Get the file.** A developer runs the extraction and gives you `messages/<your-locale>.xlf`,
   or you pull the latest from git.
2. **Translate.** Fill in every `<target>`. Preserve all `<x equiv-text="…"/>` placeholders.
3. **Hand it back / commit.** A developer runs `make i18n-build` to generate the runtime module,
   then rebuilds. Your translations now appear in the app when a user selects your locale.
4. **Iterate.** When new strings are added to the code, re-running extraction updates your
   `.xlf` file. Already-translated `<trans-unit>`s are kept, and new ones appear with empty
   `<target>` for you to fill.

---

## Tips

- **Don't translate the `id` attribute.** It's an internal key (e.g. `save_changes`), not
  display text.
- **Keep the same XML structure.** Don't merge or split `<trans-unit>` elements.
- **Mind punctuation & spacing.** French puts a space before `!`, `?`, `:`, `;`. German uses
  „quotes". These matter for polish.
- **Length differences are fine.** German/French translations are often ~30% longer than
  English; the UI is designed to accommodate this. If a string overflows badly, flag it to a
  developer rather than abbreviating awkwardly.
- **Leave `<target>` empty (or omit it) if you're unsure.** An empty target falls back to the
  English `<source>` at runtime, which is safer than a wrong translation.
- **Use a CAT tool for large files.** For > 100 strings, importing the `.xlf` into Crowdin /
  Lokalise / OmegaT / Poedit makes editing far easier than hand-editing XML.

## Common mistakes

| Mistake | Consequence |
|---|---|
| Editing `<source>` | Overwritten on next extraction |
| Removing `<x equiv-text="…"/>` | Runtime shows the placeholder text literally or crashes |
| Renaming a placeholder's `equiv-text` | String won't interpolate; shows empty or the key |
| Translating the `id` | String can't be looked up; falls back to English |
| Merging two `<trans-unit>`s | One translation is lost |
