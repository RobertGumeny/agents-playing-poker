# Heads-Up No-Limit Texas Hold'em

This document applies generic Texas Hold'em rules to the heads-up no-limit variant used by this repository.

Use [`texas-holdem.md`](texas-holdem.md) for the base game concepts. This document focuses on heads-up-specific details that are easy to implement incorrectly and that should not be left to interpretation.

## Overview

- **Heads-up** means exactly two players are seated in the hand.
- **No-limit** means a player may wager any amount from the minimum legal bet or raise up to their full remaining stack, subject to normal no-limit betting rules.

With exactly two players, every betting decision is between:

- the **button / small blind**, and
- the **big blind**.

## Button and blinds in heads-up play

In heads-up Hold'em:

- The **button posts the small blind**.
- The other player posts the **big blind**.
- The button acts **first preflop**.
- The big blind acts **first on the flop, turn, and river**.
- The button acts **last on the flop, turn, and river**.

This is the correct heads-up blind and action-order arrangement.

## Preflop state after blinds

After blinds are posted but before any voluntary preflop action:

- the small blind has already committed the small blind amount,
- the big blind has already committed the big blind amount,
- the current wager is the big blind amount,
- and the button / small blind acts first.

The amount **to call** for the button / small blind is the difference between the big blind and small blind.

If a player does not have enough chips to post a blind, that player posts their remaining stack all-in. The opponent may still complete the full blind amount if required by the blind level, and if only one player has chips remaining after that action the rest of the board is dealt with no further betting.

## Preflop action order

After the blinds are posted and hole cards are dealt:

1. The player on the **button / small blind** acts first.
2. The player in the **big blind** acts second.

### If the button / small blind folds

- The hand ends immediately.
- The big blind wins the pot.

### If the button / small blind calls

- The button / small blind adds enough chips to match the big blind.
- Action then moves to the big blind.
- The big blind may **check** or **raise**.
- The big blind cannot fold here because the big blind is not facing any additional wager after the call.
- If the big blind checks, preflop betting is closed and the hand proceeds to the flop.

### If the button / small blind raises

- The action is a raise because the player is facing the live big blind wager.
- Action then moves to the big blind.
- The big blind may fold, call, or re-raise if legally permitted.
- If the big blind calls the final preflop wager, preflop betting is closed and the hand proceeds to the flop.

## Postflop action order

On the flop, turn, and river:

1. The **big blind** acts first if still in the hand.
2. The **button** acts second if still in the hand.

This same postflop order applies on every later street.

## Legal actions in no-limit betting

In no-limit Hold'em, the legal action set depends on the current wager state and player stack.

### When no wager is facing the player

If the current wager on the street is zero, the player may:

- **check**, or
- **bet** any legal amount from the minimum opening bet up to all-in.

### When a wager is facing the player

If the player has not yet matched the current wager, the player may:

- **fold**,
- **call**, or
- **raise** any legal amount up to all-in.

A player facing a wager cannot check.

## Minimum opening bet

When no bet has yet been made on a street, the minimum opening bet is at least the size of the big blind.

## Minimum raise

A legal raise must normally increase the current wager by at least the size of the previous full bet or raise increment.

Expressed differently:

- determine the current wager,
- determine the size of the last full increase to that wager,
- then require the next non-all-in raise to increase the wager by at least that much.

Examples:

- If the current wager is 10 and a player raises to 20, the raise increment is 10.
- If the current wager is then raised to 35, the raise increment is 15.
- The next full raise must therefore reach at least 50.

### Preflop example with blinds 1/2

If blinds are 1/2:

- the small blind starts with 1 committed,
- the big blind starts with 2 committed,
- the small blind must add 1 to call,
- and the smallest legal non-all-in raise by the small blind is to a total of 4.

That raise is from a current wager of 2 to a new wager of 4, which is an increase of 2: the size of the big blind.

## Short all-in raises

An all-in wager that is less than a full legal raise:

- still counts as a call plus additional chips,
- updates the amount the opposing player would need to match if action continues,
- but does **not** by itself reopen raising to players who have already acted on that round unless the all-in amount is large enough to constitute a full raise.

In heads-up play, this means a player who has already acted on the street may sometimes be allowed to **call only** after the opponent's short all-in, rather than being allowed to raise again.

## End of betting round

In a two-player hand, a betting round ends when one of the following happens:

- one player folds,
- both remaining players have checked,
- a bet or raise is made and then matched by the other remaining player,
- or one or both players are all-in and no further action is possible.

Street transitions in heads-up play therefore look like this:

- **check / check** -> next street or showdown if on river
- **bet / call** -> next street or showdown if on river
- **bet / fold** -> hand ends immediately
- **raise / call** -> next street or showdown if on river
- **raise / fold** -> hand ends immediately

If the last aggressive action on a street is not fully called because the opponent folds, the unmatched portion of that wager is not contested and is returned to the bettor as part of normal resolution.

## Showdown in heads-up play

If both players remain after the river betting round, the hand goes to showdown.

If both players remain but all betting ended earlier because one or both players are all-in, any remaining community cards are dealt and then the hand goes to showdown.

Each player makes the best possible five-card hand from:

- their two hole cards, and
- the five community cards.

The higher-ranked hand wins the pot. If the hands are exactly tied, the pot is split equally.

## Boundary with repository-specific policy

This file is domain documentation, not project policy.

It should answer questions such as:

- who posts which blind in heads-up play,
- who acts first preflop,
- who acts first postflop,
- what amount is initially live preflop,
- how minimum bets and raises work,
- and when a betting round ends.

Repo-specific items such as wire-format field names, timeout behavior, auto-fold policy, and session artifact layout belong in [`../spec.md`](../spec.md), not here.
