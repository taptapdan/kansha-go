/** Tailwind config for kansha's own CSS. Run `just css` from the repo root. */
module.exports = {
  content: ["./web/templates/**/*.html", "./web/js/**/*.js"],
  theme: {
    extend: {
      colors: {
        bg: "var(--bg)",
        fg: "var(--fg)",
        muted: "var(--muted)",
        faint: "var(--faint)",
        line: "var(--line)",
        panel: "var(--panel)",
        code: "var(--code)",
        accent: "var(--accent)",
        accentsoft: "var(--accent-soft)",
        warn: "var(--warn)",
        warnsoft: "var(--warn-soft)",
        danger: "var(--danger)",
        dangersoft: "var(--danger-soft)",
        ok: "var(--ok)",
      },
      fontFamily: {
        sans: ["Geist", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ['"Geist Mono"', "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
};
