// The main-process composition root intentionally stays tiny. Domain setup and
// lifecycle wiring live in app.ts; this file is the ESM entrypoint bundled for Electron.
import './app.js';
