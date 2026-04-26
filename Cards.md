> **Disruption type rule (applies to all orange cards):** When played as a response during the opponent's ignition window (`ignite_reaction`), the player must banish 1 Power card from their hand as an additional cost. When played on the player's own turn (targeting a card already in the opponent's ignition slot from a previous turn), no extra cost is required.

# Knight Touch
## Type: Power
## Description
Give any piece you control, except the king or a knight, the ability to move as if it were a knight for one turn.
## Cost: 3
## Ignition: 0
## Cooldown: 2
## Targets: 1
## Effect duration: 1
## Example:
```
1. You have a pawn on e4.
2. You activate "Knight Touch".
3. You move the pawn to f6.
```

# Double Turn
## Type: Power
## Description
Give yourself 1 extra move for one turn.
## Cost: 6
## Ignition: 2
## Cooldown: 9
## Targets: 0
## Effect duration: 1
## Example:
```
1. You have a pawn on e4.
2. You activate "Double Turn".
3. Ignition succeeds 2 turns from now.
4. You move the pawn to e5.
5. Then you capture a pawn on f6 with the pawn on e5.
```

# Mana Burn
## Type: Retribution
## Description
Burn x mana from your opponent. x is the mana cost of the card in your opponent's ignition slot. If your opponent doesn't have enough mana, the excess is burned from their energized mana pool.
## Cost: 1
## Ignition: 0
## Cooldown: 3
## Targets: 0
## Effect duration: 0
## Example:
```
1. Opponent activates "Knight Touch" (cost: 3).
2. You activate "Mana Burn" as retribution.
3. Opponent has 2 mana: 2 mana is burned from their mana pool, 1 from their energized mana pool.
```

# Energy Gain
## Type: Power
## Description
Gain 4 mana.
## Cost: 0
## Ignition: 1
## Cooldown: 2
## Targets: 0
## Effect duration: 0
## Example:
```
1. You currently have 2 mana.
2. You activate "Energy Gain".
3. Ignition succeeds the next turn.
4. You gain 4 mana.
```

# Piece Swap
## Type: Power
## Description
Swap positions of one piece you control with one piece your opponent controls up to 2 squares apart, except the king.
## Cost: 6
## Ignition: 0
## Cooldown: 6
## Targets: 2
## Effect duration: 0
## Example:
```
1. You control a pawn on a3.
2. You activate "Piece Swap".
3. Your opponent controls a rook on a1.
4. You swap the pawn on a3 with the rook on a1.
5. The pawn on a3 is now on a1 and the rook on a1 is now on a3.
6. The pawn now on a1 can promote.
```

# Mind Control
## Type: Power
## Description
Target a piece your opponent controls, except the king or a queen, and take control of it for three turns.
## Cost: 7
## Ignition: 2
## Cooldown: 10
## Targets: 1
## Effect duration: 3
## Example:
```
1. You activate "Mind Control".
2. Ignition succeeds 2 turns later.
3. Opponent controls a rook on a1.
4. You take control of the rook on a1 for 3 turns.
```

# Zip Line
## Type: Power
## Description
Target a piece you control, except the king, move it to an empty square in the same row. This will use the turn movement.
## Cost: 4
## Ignition: 0
## Cooldown: 4
## Targets: 1
## Effect duration: 0
## Example:
```
1. You control a bishop on b2.
2. You activate "Zip Line".
3. The bishop on b2 moves to g2.
```

# Sacrifice of the Masses
## Type: Power
## Description
Target a pawn you control, sacrifice it to gain 6 mana and draw 2 cards.
## Cost: 0
## Ignition: 0
## Cooldown: 10
## Targets: 1
## Effect duration: 0
## Example:
```
1. You control a pawn on a3.
2. You activate "Sacrifice of the Masses".
3. The pawn on a3 is sacrificed and you gain 6 mana and draw 2 cards.
```

# Archmage Arsenal
## Type: Power
## Description
Search your deck for a "Power" card that costs 3 mana or less, except "Archmage Arsenal", add it to your hand.
## Cost: 1
## Ignition: 0
## Cooldown: 2
## Targets: 0
## Effect duration: 0
## Example:
```
1. You activate "Archmage Arsenal".
2. You search for "Knight Touch" in your deck and add it to your hand.
```

# Rook Touch
## Type: Power
## Description
Give any piece you control, except the king or a rook, the ability to move as if it were a rook for one turn. If the target is a pawn, it may move only 1 square when using this movement.
## Cost: 3
## Ignition: 0
## Cooldown: 2
## Targets: 1
## Effect duration: 1
## Example:
```
1. You have a knight on b2.
2. You activate "Rook Touch".
3. You move the knight to b7.
```

# Bishop Touch
## Type: Power
## Description
Give any piece you control, except the king or a bishop, the ability to move as if it were a bishop for one turn. If the target is a pawn, it may move only 1 square when using this movement.
## Cost: 3
## Ignition: 0
## Cooldown: 2
## Targets: 1
## Effect duration: 1
## Example:
```
1. You have a rook on b2.
2. You activate "Bishop Touch".
3. You move the rook to c3.
```

# Retaliate
## Type: Retribution
## Description
Target a "Power" card on your opponent's cooldown slot, burn x mana from your opponent, where x is the mana cost of the targeted card, and if you do, activate that card's effect for yourself.
## Cost: 2
## Ignition: 0
## Cooldown: 9
## Targets: 0
## Effect duration: 0
## Example:
```
1. Opponent has a "Knight Touch" on their cooldown slot.
2. Opponent activates any "Power" card.
3. You activate "Retaliate" as retribution.
4. You target the opponent's "Knight Touch", burning 3 mana from your opponent successfully.
5. You then activate the "Knight Touch" for yourself, buffing your rook on a1.
6. On your next turn you move the rook on a1 to c2.
```

# Counterattack
## Type: Counter
## Description
If a piece you control would be captured by a piece buffed by a "Power" card this turn, capture the attacking piece instead.
## Cost: 1
## Ignition: 0
## Cooldown: 6
## Targets: 0
## Effect duration: 0
## Example:
```
1. Opponent activates "Rook Touch" on his knight on e4.
2. Your opponent attempts to capture your queen on e6.
3. You activate "Counterattack".
4. The attacking knight is captured instead.
```

# Blockade
## Type: Counter
## Description
If a piece you control would be captured by the activation of a "Counter" card this turn while attacking, negate the effect of the "Counter" card, then return the attacking piece to its original position and choose another piece to move this turn.
## Cost: 0
## Ignition: 0
## Cooldown: 3
## Targets: 0
## Effect duration: 0
## Example:
```
1. You move your knight on e4 that was buffed by "Rook Touch" to e6 in an attempt to capture the opponent's queen.
2. Opponent activates "Counterattack" on your knight.
3. You activate "Blockade".
4. The effect of "Counterattack" is negated and your knight stays on e4.
5. You choose another piece to move this turn.
```

# Backstab
## Type: Power
## Description
Target a pawn you control that is currently facing an opponent's piece with an empty square behind it, jump over the piece and capture it, and if you do, gain 3 mana.
## Cost: 1
## Ignition: 1
## Cooldown: 7
## Targets: 0
## Effect duration: 0
## Example:
```
1. You activate "Backstab".
2. Ignition succeeds the next turn.
3. You have a pawn on e4.
4. Opponent has a knight on e5 with an empty square behind it.
5. The pawn on e4 jumps over the knight on e5 to e6 and captures it.
6. You gain 3 mana.
```

# Stop Right There!
## Type: Retribution
## Description
Target an ignited card from your opponent and negate its effect.
## Cost: 3
## Ignition: 0
## Cooldown: 5
## Targets: 0
## Effect duration: 0
## Example:
```
1. Opponent activates "Knight Touch".
2. You activate "Stop Right There!" as retribution.
3. The effect of "Knight Touch" is negated.
```

# Renewal
## Type: Power
## Description
Consume up to 10 energized mana and gain half the amount as mana.
## Cost: 0
## Ignition: 1
## Cooldown: 2
## Targets: 0
## Effect duration: 0
## Example:
```
1. You have 10 energized mana.
2. You activate "Renewal".
3. Ignition succeeds the next turn.
4. You consume 10 energized mana and gain 5 mana.
```

# Life Drain
## Type: Continuous
## Description
While on the ignition slot, every capture you make drains 1 mana from your opponent. When this card's ignition ends, banish this card.
## Cost: 3
## Ignition: 5
## Cooldown: 0
## Targets: 0
## Effect duration: 0
## Example:
```
1. You activate "Life Drain".
2. You capture any piece this turn.
3. You gain 1 mana from the capture + 1 mana from your opponent's mana pool.
```

# Clairvoyance
## Type: Continuous
## Description
While on the ignition slot, reveal your opponent's hand. When this card's ignition ends, banish this card.
## Cost: 7
## Ignition: 3
## Cooldown: 0
## Targets: 0
## Effect duration: 0
## Example:
```
1. You activate "Clairvoyance".
2. Your opponent's hand is revealed to you and stays revealed until this card's ignition ends.
```

# Thunderstorm
## Type: Power
## Description
Randomly choose 10 squares on the board, for each piece you control that is on one of the chosen squares, gain 2 mana, but they cannot move this turn, for each piece your opponent controls that is on one of the chosen squares, burn 2 mana from your opponent, and they cannot move until the end of the next turn.
## Cost: 10
## Ignition: 1
## Cooldown: 10
## Targets: 0
## Effect duration: 0
## Example:
```
1. You activate "Thunderstorm".
2. Ignition succeeds the next turn.
3. You randomly choose 10 squares on the board.
4. 3 pieces you control are hit, you gain 6 mana and they cannot move this turn.
5. 4 pieces your opponent controls are hit, you burn 8 mana from your opponent and those pieces cannot move until the end of the next turn.
```

# Extinguish
## Type: Disruption
## Description
Target a card on your opponent's ignition slot and negate its effect.
## Cost: 2
## Ignition: 0
## Cooldown: 2
## Targets: 0
## Effect duration: 0
## Example:
```
1. Opponent has a "Double Turn" in their ignition slot.
2. You activate "Extinguish".
3. The effect of "Double Turn" is negated and sent to their cooldown slot.
```

# Save It For Later
## Type: Retribution
## Description
This card can only be activated if you have a card in your ignition slot. Target that card and move it back to your hand, gaining the mana cost of the card as mana.
## Cost: 0
## Ignition: 0
## Cooldown: 10
## Targets: 0
## Effect duration: 0
## Example:
```
1. You have "Double Turn" in your ignition slot.
2. Your opponent activates "Extinguish" on your "Double Turn".
3. You activate "Save It For Later" as retribution.
4. The "Double Turn" card is moved back to your hand and you gain 6 mana.
```
