# Contributing to MessagePit

Thank you for your interest in contributing to MessagePit!

## Reporting issues and feature requests

If you find a bug or have a feature request, please [open an issue](https://github.com/coreydaley/messagepit/issues) and provide as much detail as possible. Please **do not** report security issues here (see below).

## Reporting security issues

Please do not report security issues publicly in GitHub. Use the "Report a vulnerability" button in the [Security tab](../../security/advisories/new) of this repository.

## Contributing code

Please ensure your code is clean and passes linting before submitting a Pull Request.

MessagePit is a fork of [Mailpit](https://github.com/axllent/mailpit) extended with SMS support. Contributions that enhance the SMS functionality or improve compatibility with Mailpit upstream are especially welcome.

## Frontend development

The frontend lives in `server/ui-src/` and is compiled by esbuild into `server/ui/dist/`, which is then embedded into the Go binary via `//go:embed`.

### Tech stack

- **Vue 3** with `<script setup>` (Composition API) for all list and search views
- **Bootstrap 5** for layout and components
- **Vue Router** for client-side routing
- **Axios** for HTTP requests
- **dayjs** for date formatting

### Build commands

```bash
make ui      # compile frontend assets (development, with source maps)
make package # compile frontend assets (production, minified)
make run     # compile UI + binary and run with dev defaults
npm run lint # lint with ESLint + Prettier
```

### Architecture

**Composables** (`server/ui-src/composables/`):

- `useCommon.js` — shared HTTP helpers (`get`, `post`, `put`, `del`), loading state, URL resolution, pagination parameter parsing, clipboard, tag color, and number formatting. All list and search views call `useCommon()`.
- `useMessages.js` — wraps `useCommon` and adds email message loading logic (`loadMessages`, `reloadMailbox`). Email views set `apiURI.value` then call `loadMessages()`.

**Shared layout** (`server/ui-src/components/AppLayout.vue`):

All views use `AppLayout` and provide content via named slots:

```vue
<AppLayout active-tab="email" offcanvas-id="offcanvas" :loading="loading">
  <template #search><!-- search form --></template>
  <template #sidebar><!-- sidebar nav --></template>
  <template #modals><!-- modal variants of sidebar nav --></template>
  <!-- default slot: main content area -->
</AppLayout>
```

The `#sidebar` slot renders twice (mobile offcanvas + desktop sidebar). Vue 3 calls the slot function independently each time, producing two separate DOM trees.

**Stores** (`server/ui-src/stores/`): plain reactive objects (`mailbox`, `pagination`, `smsStore`, `webhooksStore`) shared across components via direct import.

### Linting

Both ESLint and Prettier are enforced. Run `npm run lint` before committing. The project targets Vue 3 with `vue/vue3-recommended` rules; `vue/no-v-html` is disabled globally to allow safe HTML rendering in the message preview pane.
