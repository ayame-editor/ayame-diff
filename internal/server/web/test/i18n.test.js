"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { CATALOG, FALLBACK_LANGUAGE, languages, languageMeta, localeTag, direction, nextLanguage, translate, pickLanguage } = require("../i18n.js");

test("the catalog holds the shipped languages", () => {
  assert.deepEqual(languages().sort(), ["en", "ja"]);
  assert.ok(CATALOG[FALLBACK_LANGUAGE], "the fallback language has no table");
});

test("translate returns strings and calls message functions", () => {
  assert.equal(translate(CATALOG, "en", "compare"), "Compare");
  assert.equal(translate(CATALOG, "ja", "compare"), "比較");
  assert.equal(
    translate(CATALOG, "en", "needPaths", { fields: "LEFT / RIGHT" }),
    "Specify LEFT / RIGHT",
  );
});

test("an unknown key shows itself instead of an empty string", () => {
  assert.equal(translate(CATALOG, "en", "noSuchKey"), "noSuchKey");
});

test("an unknown language falls back rather than throwing", () => {
  assert.equal(translate(CATALOG, "fr", "compare"), translate(CATALOG, FALLBACK_LANGUAGE, "compare"));
});

test("a stored choice wins over the browser's language", () => {
  assert.equal(pickLanguage("ja", "en-US"), "ja");
  assert.equal(pickLanguage("en", "ja-JP"), "en");
});

test("without a stored choice the browser language decides", () => {
  assert.equal(pickLanguage(null, "ja"), "ja");
  assert.equal(pickLanguage(null, "ja-JP"), "ja");
  assert.equal(pickLanguage("", "JA-jp"), "ja");
  assert.equal(pickLanguage(undefined, "en-GB"), "en");
});

test("an unsupported request falls back to English", () => {
  assert.equal(pickLanguage("de", "de-DE"), "en");
  assert.equal(pickLanguage(null, ""), "en");
  assert.equal(pickLanguage(null, undefined), "en");
  // A tag that merely starts with a language's letters is not that language.
  assert.equal(pickLanguage(null, "java"), "en");
});

test("every language carries its own name, locale tag and direction", () => {
  for (const code of languages()) {
    const meta = languageMeta(code);
    assert.equal(typeof meta.name, "string");
    assert.ok(meta.name.length > 0, `${code} has no display name`);
    assert.equal(typeof meta.tag, "string");
    assert.ok(["ltr", "rtl"].includes(meta.dir), `${code} has an unknown direction`);
    assert.equal(localeTag(code), meta.tag);
    assert.equal(direction(code), meta.dir);
  }
  assert.equal(languageMeta("ja").name, "日本語");
  assert.equal(languageMeta("en").name, "English");
  assert.equal(localeTag("ja"), "ja-JP");
  assert.equal(direction("en"), "ltr");
});

test("an unknown language still gets usable metadata", () => {
  const meta = languageMeta("ar");
  assert.equal(meta.name, "ar");
  assert.equal(meta.tag, "ar");
  assert.equal(meta.dir, "ltr");
});

test("the switcher advances through catalog order and wraps", () => {
  assert.equal(nextLanguage("ja"), "en");
  assert.equal(nextLanguage("en"), "ja");
  assert.equal(nextLanguage("fr"), languages()[0]);
});
