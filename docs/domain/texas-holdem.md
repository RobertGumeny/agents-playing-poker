# Texas Hold'em

This document is the canonical rules reference for generic Texas Hold'em used by this repository.

It describes facts that are true because of the game itself, not because of this project's implementation. For project-specific constraints or overrides, see [`../spec.md`](../spec.md). For the documentation split, see [`README.md`](README.md).

## Scope of this document

This file defines base Hold'em concepts that should not be re-invented by agents or engine code.

If a required rule is not explicit here, that is a documentation gap to fix rather than a prompt for implementation guesswork.

Variant-specific details such as exact heads-up blind assignment and action order belong in a more specific domain document such as [`heads-up-nlhe.md`](heads-up-nlhe.md).

## Overview

Texas Hold'em is a community-card poker variant played with a standard 52-card deck. Each player is dealt two private cards, and five shared community cards are dealt face up on the board by the end of the hand unless the hand ends earlier.

A player's final hand is the best five-card poker hand that can be made from any combination of:

- their two hole cards, and
- the five community cards.

Only the best five-card hand matters. Extra cards outside that best five-card combination do not affect comparison.

## Cards and terminology

- **Deck**: a standard 52-card deck with four suits and thirteen ranks.
- **Ranks**: `2 3 4 5 6 7 8 9 T J Q K A`.
- **Rank order**: ace is high in ordinary rank comparison, except that it may be used as the low card in the straight `A-2-3-4-5`.
- **Suits**: clubs, diamonds, hearts, spades.
- **Hole cards**: the two private cards dealt to each player.
- **Board** or **community cards**: shared face-up cards available to all remaining players.
- **Burn card**: a face-down card discarded before dealing community cards in live dealing procedure. Burn-card procedure does not affect hand ranking semantics.
- **Button** or **dealer button**: marker showing the nominal dealer position for the hand.
- **Small blind** and **big blind**: forced bets posted before preflop action begins.
- **Pot**: the total chips wagered in the hand.
- **Street**: one betting phase of the hand: preflop, flop, turn, or river.
- **Current wager**: the amount a player must have committed on the current street to be fully matched.
- **To call**: the amount a player must add on the current street to match the current wager.
- **Showdown**: resolution of a hand when multiple players remain after the final betting round or after all remaining betting action has become impossible.

## Structure of a hand

A standard hand of Texas Hold'em proceeds in this order:

1. Button and blinds are assigned.
2. Hole cards are dealt to each player.
3. **Preflop** betting round.
4. **Flop**: three community cards are dealt face up.
5. **Flop** betting round.
6. **Turn**: one additional community card is dealt face up.
7. **Turn** betting round.
8. **River**: one final community card is dealt face up.
9. **River** betting round.
10. If more than one player remains, showdown determines the winner.

A hand ends immediately if all but one player fold.

If one or more players are all-in and no further betting action is possible, any remaining community cards are dealt and the hand proceeds to showdown.

## Betting rounds

A betting round starts with a defined first player to act and continues until the betting is closed.

Betting is closed when one of the following is true:

- only one player remains because all others folded,
- every remaining player has matched the current wager,
- every remaining player has checked because no wager was made on that street,
- or all remaining players are all-in and no further action is possible.

Community cards by street:

- **Preflop**: no board cards yet.
- **Flop**: first three board cards.
- **Turn**: fourth board card.
- **River**: fifth board card.

The exact first player to act depends on the game format and number of players. Variant-specific seating and action-order details should be documented separately rather than assumed.

## Legal actions

A player's legal actions depend on whether they are facing a wager and on the betting structure in use.

Core action vocabulary:

- **Fold**: surrender the hand and forfeit any claim to the pot.
- **Check**: decline to bet when no wager is facing the player.
- **Bet**: place the first voluntary wager on a betting round when the current wager is zero.
- **Call**: match the current wager by adding exactly the amount needed to do so, or by putting in the player's remaining chips if that stack is smaller than the full call amount.
- **Raise**: increase the current wager after a bet or raise has already been made.
- **All-in**: commit all remaining chips; this may be as a bet, call, or raise depending on the situation.

General constraints:

- A player cannot check when facing a live wager.
- A player cannot bet if a live wager already exists on that street; in that case the aggressive action is a raise.
- A player cannot call more chips than they have remaining.
- A raise must meet the minimum raise requirement for the betting structure unless the player is all-in for less.
- A player with no chips remaining can take no further betting actions in that hand.

Operational notes:

- Chips committed on earlier streets do not count toward the amount needed to call on a later street; each street starts a new betting round.
- Within a street, the amount a player has already committed on that street does count toward matching the current wager.
- If a player makes a wager and no opponent matches the full amount because the opponent folds, any uncalled portion is not contested at showdown; it is returned to the bettor as part of normal pot resolution.

## Hand rankings

From highest to lowest, standard poker hand rankings are:

1. **Royal flush**: `A-K-Q-J-T` all of the same suit.
2. **Straight flush**: five consecutive cards of the same suit.
3. **Four of a kind**: four cards of the same rank.
4. **Full house**: three cards of one rank and two cards of another rank.
5. **Flush**: five cards of the same suit, not all consecutive.
6. **Straight**: five consecutive cards, not all of the same suit.
7. **Three of a kind**: three cards of the same rank.
8. **Two pair**: two cards of one rank, two cards of another rank.
9. **One pair**: two cards of the same rank.
10. **High card**: none of the above.

### Ranking notes

- Aces can be high or low in a straight, but not both at once. `A-K-Q-J-T` and `5-4-3-2-A` are valid straights; `K-A-2-3-4` is not.
- Suits do not break ties in standard Hold'em.
- When players have the same hand category, comparison is by the ranked cards that define that category and then by kickers where relevant.

### Category tie-break rules

- **Royal flush**: always tied against another royal flush.
- **Straight flush**: compare the highest card in the straight.
- **Four of a kind**: compare the rank of the four matching cards, then the kicker.
- **Full house**: compare the rank of the three-card group, then the rank of the pair.
- **Flush**: compare the highest card, then next highest, and so on through all five cards.
- **Straight**: compare the highest card in the straight.
- **Three of a kind**: compare the rank of the trips, then the highest kicker, then the next kicker.
- **Two pair**: compare the higher pair, then the lower pair, then the kicker.
- **One pair**: compare the pair, then the highest kicker, then the next, then the next.
- **High card**: compare the highest card, then next highest, and so on through all five cards.

## Showdown and winner determination

At showdown, each remaining player forms the best possible five-card hand from any combination of their two hole cards and the five community cards.

A player may use:

- both hole cards,
- one hole card,
- or none of their hole cards if the board itself is their best hand.

All remaining players compare hands using the standard ranking rules above.

- The winner is the player with the highest-ranked five-card hand.
- If two or more players have exactly equal five-card hands, the pot is split equally among them under normal poker rules.
- If all other players fold before showdown, the last remaining player wins the pot without needing to show a winning hand.

## Remaining boundary cases

Most core Hold'em rules needed by the engine should be explicit in this document and in [`heads-up-nlhe.md`](heads-up-nlhe.md).

One class of behavior commonly varies by house rule rather than by the game itself: exact handling of an **odd chip** when an integer-valued pot cannot be split evenly. If the repository's chip model needs a deterministic odd-chip rule, that rule should be stated explicitly in project policy or in a variant-specific domain doc rather than guessed.
