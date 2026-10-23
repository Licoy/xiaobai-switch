# Native icon sources

- General UI icons are unmodified SVG files from official npm `remixicon@4.9.0`, https://github.com/Remix-Design/RemixIcon / https://remixicon.com/ ，prefer the outlined `-line` variants (only `pushpin-fill` marks the pinned tray panel)，license preserved in `REMIX-LICENSE`
- ClaudeCode, Codex and Pi SVG paths are extracted without drawing changes from installed official `@lobehub/icons@5.15.0` `es/{ClaudeCode,Codex,Pi}/components/Mono.js`，https://github.com/lobehub/lobe-icons ，MIT license preserved in `LOBE-LICENSE`
- Prime has no dedicated icon in that fixed Lobe package，uses the general Remix `sparkling-line` symbol，this does not claim to be a Prime brand mark
- Runtime embeds SVG assets only，no React runtime is introduced to the native app
