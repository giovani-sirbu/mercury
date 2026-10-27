// Package tradelog writes the non-action rows of Trade.Logs, the entries a
// chain leaves on a trade when it stops instead of acting. It writes two:
//
//   - the ERROR entry (SaveError) an exchange or funds failure leaves on the
//     trade, with the exchange error text normalized so repeats collapse;
//   - the INFO hold row (SaveHoldLog) a gate's refusal leaves, with its
//     strategy event beside it when the hold names a strategy param: the event
//     exists exactly when its row does, and a hold that repeats is written
//     again only once its row is older than holdRelogAfter.
//
// The package imports the gates root for the Hold SaveHoldLog takes; that root
// never imports it.
package tradelog
