---
paths:
  - "src/screens/**/*"
  - "src/components/**/*"
  - "src/store/**/*"
  - "src/api/**/*"
  - "src/navigation/**/*"
---

# Frontend & UI Rules (hermatic — React Native)

## App Architecture

```
Screens → Hooks → Redux (actions/sagas/selectors) → API layer → Backend services
               → Components (pure UI, receive props)
```

---

## Folder Structure Rules

### CRITICAL: Screens organized by flow
```
screens/
  AuthFlow/
    SignIn/
      index.tsx              # Screen component
      components/            # Screen-specific components
        SignInForm/index.tsx
      hooks/                 # Screen-specific hooks
        useEmailAuth.ts
  HomeFlow/
    Home/
      index.tsx
      components/
        ActiveTrades/
          index.tsx
          hooks/
            useActiveTrades.ts
      hooks/
        useTradesError.tsx
```

### CRITICAL: One component per file
`index.tsx` in each component folder. File name matches component folder name.

### IMPORTANT: Hooks colocated with their consumer
Screen-specific hooks live in the screen's `hooks/` folder. Shared hooks live in `src/hooks/`.

### IMPORTANT: Store modules follow the Redux pattern
Every store module has:
```
store/modules/[Feature]/
  actions/       # Action creators + types
  reducer/       # Reducer + initial state
  sagas/         # Side effects (API calls, async logic)
  selectors/     # Memoized selectors
```

---

## React Native Conventions

### Components
- Components are pure UI. They receive data and callbacks via props.
- Screens orchestrate: fetch data (via hooks/Redux), handle interactions, pass props to components.
- Shared components live in `src/components/`. Screen-specific components live in the screen's `components/` folder.

```tsx
// Screen — orchestrates data and passes to components
function Home() {
  const trades = useActiveTrades();
  const handleStop = (tradeId) => dispatch(stopTrade(tradeId));
  return <ActiveTrades trades={trades} onStop={handleStop} />;
}

// Component — pure UI
function ActiveTrades({ trades, onStop }: Props) {
  // renders UI only, no business logic
}
```

### Styling
- Use `StyleSheet.create()` for all styles.
- Use constants from `src/constants/colors.ts`, `fonts.ts`, `styles.ts`.
- No magic numbers — extract to constants.

### State Management
| Type | Tool |
|---|---|
| Server/async state | Redux + Redux Saga |
| Persisted state | redux-persist |
| Navigation state | React Navigation |
| Local UI state | `useState` / `useReducer` |

### Navigation
- React Navigation with native-stack and bottom-tabs.
- Auth flow and Main flow are separate navigators (see `navigation/Auth.tsx`, `navigation/Main.tsx`).
- Screen components in `src/screens/[Flow]/[Screen]/`.

---

## API Layer

### CRITICAL: All HTTP calls go through `src/api/`
No raw `axios` calls in screens, components, or hooks. API modules are the boundary.

```
api/
  client/              # Axios instance with interceptors (auth token injection, refresh)
    interceptors/      # Request/response interceptors
  auth/                # login, refreshToken
  trades/              # createTrade, getTrade, pauseTrade, stopTrade, etc.
  exchanges/           # addExchange, getExchanges, deleteExchange, etc.
  settings/            # getSettings, getAppVersion, etc.
  stats/               # getDashboardStats, getBestPairs, etc.
  user/                # createAccount, getSession, deleteAccount, etc.
```

### IMPORTANT: API modules match backend service routes
Each API module corresponds to a backend service endpoint group. Keep them in sync.

### IMPORTANT: Auth token management
- JWT stored in react-native-keychain (secure storage)
- Axios interceptor injects auth header automatically
- Token refresh handled transparently in interceptor

---

## Redux Saga Patterns

### IMPORTANT: Side effects in sagas, not in components
API calls, navigation after async operations, and complex workflows live in sagas. Components dispatch actions and read selectors.

```tsx
// saga — handles async
function* createTradeSaga(action) {
  try {
    const response = yield call(tradesApi.createTrade, action.payload);
    yield put(createTradeSuccess(response));
  } catch (error) {
    yield put(createTradeFailure(error));
  }
}

// component — dispatches and reads
const dispatch = useDispatch();
dispatch(createTradeRequest(payload));
const loading = useSelector(selectTradesLoading);
```

### IMPORTANT: Actions follow the Request/Success/Failure pattern
```ts
createTradeRequest  → triggers saga
createTradeSuccess  → updates reducer with result
createTradeFailure  → updates reducer with error
```

---

## Security

### CRITICAL: Never store sensitive data in AsyncStorage
Use `react-native-keychain` for JWT tokens, exchange credentials, and any sensitive data.

### CRITICAL: Never log or display exchange API keys
Exchange credentials are sensitive. Never show full keys in the UI — mask them.

### IMPORTANT: Config via react-native-config
Environment-specific values (API URLs, feature flags) come from `.env.app` via `react-native-config`. Never hardcode backend URLs.

---

## Performance

### IMPORTANT: FlatList for all lists
Never use `ScrollView` with `.map()` for lists. Use `FlatList` with proper `keyExtractor`.

### IMPORTANT: Minimize re-renders
- Use `React.memo()` for list items and expensive components
- Use `useCallback()` for functions passed as props
- Use `useMemo()` for expensive computations
- Use memoized Redux selectors

### STANDARD: Test on both iOS and Android
Every UI change must be tested on both platforms.

---

## Naming

### STANDARD: Component folders in PascalCase
`ActiveTrades/`, `SignInForm/`, `PairListItem/`.

### STANDARD: Hook files in camelCase with `use` prefix
`useActiveTrades.ts`, `useEmailAuth.ts`, `useStats.ts`.

### STANDARD: Store module folders in PascalCase
`Account/`, `Trades/`, `Exchanges/`, `Statistics/`.
