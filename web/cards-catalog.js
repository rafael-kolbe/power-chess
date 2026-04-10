/**
 * Card text for UI (footer marquee, future hand UI). Gameplay rules remain authoritative on the server.
 * The `EN` object must match `internal/gameplay/cards.go` `InitialCardCatalog()` (Description + Example + Name) byte-for-byte in English.
 * Portuguese (`PT`) mirrors that rules text using fixed TCG terms: alvo, oponente, mana, ignição, recarga, slot de ignição, slot de recarga, banir, mão, deck, cemitério.
 */

const CARD_ROWS = [
  { id: "knight-touch", type: "power", mana: 3, ignition: 0, cooldown: 2 },
  { id: "double-turn", type: "power", mana: 4, ignition: 1, cooldown: 5 },
  { id: "mana-burn", type: "retribution", mana: 1, ignition: 0, cooldown: 3 },
  { id: "energy-gain", type: "power", mana: 0, ignition: 1, cooldown: 2 },
  { id: "piece-swap", type: "power", mana: 6, ignition: 1, cooldown: 6 },
  { id: "mind-control", type: "power", mana: 7, ignition: 2, cooldown: 10 },
  { id: "zip-line", type: "power", mana: 4, ignition: 0, cooldown: 4 },
  { id: "sacrifice-of-the-masses", type: "power", mana: 0, ignition: 0, cooldown: 10 },
  { id: "archmage-arsenal", type: "power", mana: 1, ignition: 0, cooldown: 2 },
  { id: "rook-touch", type: "power", mana: 3, ignition: 0, cooldown: 2 },
  { id: "bishop-touch", type: "power", mana: 3, ignition: 0, cooldown: 2 },
  { id: "retaliate", type: "retribution", mana: 2, ignition: 0, cooldown: 9 },
  { id: "counterattack", type: "counter", mana: 1, ignition: 0, cooldown: 6 },
  { id: "blockade", type: "counter", mana: 0, ignition: 0, cooldown: 3 },
  { id: "backstab", type: "power", mana: 1, ignition: 1, cooldown: 7 },
  { id: "stop-right-there", type: "retribution", mana: 3, ignition: 0, cooldown: 5 },
  { id: "renewal", type: "power", mana: 0, ignition: 1, cooldown: 2 },
  { id: "life-drain", type: "continuous", mana: 3, ignition: 5, cooldown: 0 },
  { id: "clairvoyance", type: "continuous", mana: 7, ignition: 3, cooldown: 0 },
  { id: "thunderstorm", type: "power", mana: 10, ignition: 1, cooldown: 10 },
  { id: "extinguish", type: "power", mana: 2, ignition: 0, cooldown: 2 },
  { id: "save-it-for-later", type: "retribution", mana: 0, ignition: 0, cooldown: 10 }
];

/** @type {Record<string, { name: string, description: string, example: string }>} */
const EN = {
  "knight-touch": {
    name: "Knight Touch",
    description:
      "Give any piece you control, except the king or a knight, the ability to move as if it were a knight for one turn.",
    example:
      "1. You have a pawn on e4.\n2. You activate \"Knight Touch\".\n3. You move the pawn to f6."
  },
  "double-turn": {
    name: "Double Turn",
    description: "Give yourself 1 extra move for one turn.",
    example:
      "1. You have a pawn on e4.\n2. You activate \"Double Turn\".\n3. Ignition succeeds the next turn.\n4. Next turn you move the pawn to e5.\n5. Then you capture a pawn on f6 with the pawn on e5."
  },
  "mana-burn": {
    name: "Mana Burn",
    description:
      "Target an ignited card from your opponent, burn x mana from your opponent, x being the mana cost of the target card.",
    example:
      "1. Opponent activates \"Knight Touch\"\n2. You activate Mana Burn as retribution.\n3. You burn 3 mana from your opponent."
  },
  "energy-gain": {
    name: "Energy Gain",
    description: "Gain 4 mana.",
    example:
      "1. You currently have 2 mana.\n2. You activate \"Energy Gain\".\n3. Ignition succeeds the next turn.\n4. You gain 4 mana."
  },
  "piece-swap": {
    name: "Piece Swap",
    description:
      "Swap positions of one piece you control with one piece your opponent controls up to 2 squares apart, except the king.",
    example:
      "1. You control a pawn on a3.\n2. You activate \"Piece Swap\".\n3. Ignition succeeds the next turn.\n4. Opponent controls a rook on a1.\n5. Next turn you swap the pawn on a3 with the rook on a1.\n6. The pawn on a3 is now on a1 and the rook on a1 is now on a3.\n7. The pawn now on a1 can promote."
  },
  "mind-control": {
    name: "Mind Control",
    description:
      "Target a piece your opponent controls, except the king or a queen, and take control of it for three turns. If the piece is captured, move it to your opponent's graveyard.",
    example:
      "1. You activate \"Mind Control\".\n2. Ignition succeeds 2 turns later.\n3. Opponent controls a rook on a1.\n4. You take control of the rook on a1 for 3 turns."
  },
  "zip-line": {
    name: "Zip Line",
    description: "Target a piece you control, except the king, move it to an empty square in the same row.",
    example: "1. You control a bishop on b2.\n2. You activate \"Zip Line\".\n3. The bishop on b2 moves to b7."
  },
  "sacrifice-of-the-masses": {
    name: "Sacrifice of the Masses",
    description: "Target a pawn you control, sacrifice it to gain 6 mana and draw 2 cards.",
    example:
      "1. You control a pawn on a3.\n2. You activate \"Sacrifice of the Masses\".\n3. The pawn on a3 is sacrificed and you gain 6 mana and draw 2 cards."
  },
  "archmage-arsenal": {
    name: "Archmage Arsenal",
    description:
      "Search your deck for a \"Power\" card that costs 3 mana or less, except \"Archmage Arsenal\", add it to your hand.",
    example:
      "1. You activate \"Archmage Arsenal\".\n2. You search for \"Knight Touch\" in your deck and add it to your hand."
  },
  "rook-touch": {
    name: "Rook Touch",
    description:
      "Give any piece you control, except the king or a rook, the ability to move as if it were a rook for one turn.",
    example: "1. You have a knight on b2.\n2. You activate \"Rook Touch\".\n3. You move the knight to b7."
  },
  "bishop-touch": {
    name: "Bishop Touch",
    description:
      "Give any piece you control, except the king or a bishop, the ability to move as if it were a bishop for one turn.",
    example: "1. You have a rook on b2.\n2. You activate \"Bishop Touch\".\n3. You move the rook to c3."
  },
  "retaliate": {
    name: "Retaliate",
    description:
      "Target a card on your opponent's cooldown slot, burn x mana from your opponent, x being the mana cost of the targeted card, and if you do, activate that card (in your ignition slot) for yourself.",
    example:
      "1. Opponent has a \"Knight Touch\" on their cooldown slot.\n2. Opponent activates any \"Power\" card.\n3. You activate \"Retaliate\" as retribution.\n4. You target the opponent's \"Knight Touch\", burning 3 mana from your opponent successfully.\n5. You then activate the \"Knight Touch\" for yourself, buffing your rook on a1.\n6. On your next turn you move the rook on a1 to c2."
  },
  "counterattack": {
    name: "Counterattack",
    description:
      "If a piece you control would be captured by a piece buffed by a \"Power\" card this turn, capture the attacking piece instead.",
    example:
      "1. Opponent activates \"Rook Touch\" on his pawn on e4.\n2. Your opponent attempts to capture your queen on e6.\n3. You activate \"Counterattack\".\n4. The attacking pawn is captured instead."
  },
  "blockade": {
    name: "Blockade",
    description:
      "If a piece you control would be captured by the activation of a \"Counter\" card this turn while attacking, negate the effect of the \"Counter\" card, then return the attacking piece to its original position and choose another piece to move this turn.",
    example:
      "1. You move your pawn on e4 that was buffed by \"Rook Touch\" to e6 in an attempt to capture the opponent's queen.\n2. Opponent activates \"Counterattack\" on your pawn.\n3. You activate \"Blockade\".\n4. The effect of \"Counterattack\" is negated and your pawn stays on e4.\n5. You choose another piece to move this turn."
  },
  "backstab": {
    name: "Backstab",
    description:
      "Target a pawn you control that is currently facing an opponent's piece with an empty square behind it, jump over the piece and capture it, and if you do, gain 3 mana.",
    example:
      "1. You activate \"Backstab\".\n2. Ignition succeeds the next turn.\n3. You have a pawn on e4.\n4. Opponent has a knight on e5 with an empty square behind it.\n5. The pawn on e4 jumps over the knight on e5 to e6 and captures it.\n6. You gain 3 mana."
  },
  "stop-right-there": {
    name: "Stop Right There!",
    description: "Target an ignited card from your opponent and negate its effect.",
    example:
      "1. Opponent activates \"Knight Touch\"\n2. You activate \"Stop Right There!\" as retribution.\n3. The effect of \"Knight Touch\" is negated."
  },
  "renewal": {
    name: "Renewal",
    description: "Consume up to 10 energized mana and gain half the amount as mana.",
    example:
      "1. You have 10 energized mana.\n2. You activate \"Renewal\".\n3. Ignition succeeds the next turn.\n4. You consume 10 energized mana and gain 5 mana."
  },
  "life-drain": {
    name: "Life Drain",
    description:
      "While on the ignition slot, every capture you make drains 1 mana from your opponent. When this card's ignition ends, banish this card.",
    example:
      "1. You activate \"Life Drain\".\n2. You capture any piece this turn.\n3. You gain 1 mana from the capture + 1 mana from your opponent's mana pool."
  },
  "clairvoyance": {
    name: "Clairvoyance",
    description:
      "While on the ignition slot, reveal your opponent's hand. When this card's ignition ends, banish this card.",
    example:
      "1. You activate \"Clairvoyance\".\n2. Your opponent's hand is revealed to you and stays revealed until this card's ignition ends."
  },
  "thunderstorm": {
    name: "Thunderstorm",
    description:
      "Randomly choose 10 squares on the board, for each piece you control that is on one of the chosen squares, gain 2 mana, but they cannot move this turn, for each piece your opponent controls that is on one of the chosen squares, burn 2 mana from your opponent, and they cannot move their next turn.",
    example:
      "1. You activate \"Thunderstorm\".\n2. Ignition succeeds the next turn.\n3. You randomly choose 10 squares on the board.\n4. 3 pieces you control are hit, you gain 6 mana and they cannot move this turn.\n5. 4 pieces your opponent controls are hit, you burn 8 mana from your opponent and those pieces cannot move their next turn."
  },
  "extinguish": {
    name: "Extinguish",
    description: "Target a card on your opponent's ignition slot and negate its effect.",
    example:
      "1. Opponent has a \"Double Turn\" in their ignition slot.\n2. You activate \"Extinguish\".\n3. The effect of \"Double Turn\" is negated and sent to their cooldown slot."
  },
  "save-it-for-later": {
    name: "Save It For Later",
    description:
      "This card can only be activated if you have a card in your ignition slot. Target that card and move it back to your hand, gaining the mana cost of the card as mana.",
    example:
      "1. You have \"Double Turn\" in your ignition slot.\n2. Your opponent activates \"Extinguish\" on your \"Double Turn\".\n3. You activate \"Save It For Later\" as retribution.\n4. The \"Double Turn\" card is moved back to your hand and you gain 4 mana."
  }
};

/** @type {Record<string, { name: string, description: string, example: string }>} */
const PT = {
  "knight-touch": {
    name: "Toque do Cavalo",
    description:
      "Conceda a qualquer peça que você controla, exceto o rei ou um cavalo, a habilidade de se mover como se fosse um cavalo por um turno.",
    example:
      "1. Você tem um peão em e4.\n2. Você ativa \"Toque do Cavalo\".\n3. Você move o peão para f6."
  },
  "double-turn": {
    name: "Turno Duplo",
    description: "Conceda a si mesmo 1 jogada extra por um turno.",
    example:
      "1. Você tem um peão em e4.\n2. Você ativa \"Turno Duplo\".\n3. A ignição resolve no turno seguinte.\n4. No turno seguinte você move o peão para e5.\n5. Então você captura um peão em f6 com o peão em e5."
  },
  "mana-burn": {
    name: "Queimadura de Mana",
    description:
      "Alvo: uma carta ignitada do oponente. Queime x de mana do oponente, sendo x o custo de mana da carta alvo.",
    example:
      "1. O oponente ativa \"Toque do Cavalo\"\n2. Você ativa Queimadura de Mana como retribuição.\n3. Você queima 3 de mana do oponente."
  },
  "energy-gain": {
    name: "Ganho de Energia",
    description: "Ganhe 4 de mana.",
    example:
      "1. Atualmente você tem 2 de mana.\n2. Você ativa \"Ganho de Energia\".\n3. A ignição resolve no turno seguinte.\n4. Você ganha 4 de mana."
  },
  "piece-swap": {
    name: "Troca de Peças",
    description:
      "Troque as posições de uma peça sua com uma peça do oponente que estejam a até 2 casas de distância, exceto o rei.",
    example:
      "1. Você controla um peão em a3.\n2. Você ativa \"Troca de Peças\".\n3. A ignição resolve no turno seguinte.\n4. O oponente controla uma torre em a1.\n5. No turno seguinte você troca o peão em a3 com a torre em a1.\n6. O peão que estava em a3 está agora em a1 e a torre que estava em a1 está agora em a3.\n7. O peão agora em a1 pode promover."
  },
  "mind-control": {
    name: "Controle Mental",
    description:
      "Alvo: uma peça que o oponente controla, exceto o rei ou uma dama, e assuma o controle dela por três turnos. Se a peça for capturada, mova-a para o cemitério do oponente.",
    example:
      "1. Você ativa \"Controle Mental\".\n2. A ignição resolve 2 turnos depois.\n3. O oponente controla uma torre em a1.\n4. Você assume o controle da torre em a1 por 3 turnos."
  },
  "zip-line": {
    name: "Tirolesa",
    description:
      "Alvo: uma peça sua, exceto o rei; mova-a para uma casa vazia na mesma fileira (linha).",
    example:
      "1. Você controla um bispo em b2.\n2. Você ativa \"Tirolesa\".\n3. O bispo em b2 move para b7."
  },
  "sacrifice-of-the-masses": {
    name: "Sacrifício das Massas",
    description:
      "Alvo: um peão seu; sacrifique-o para ganhar 6 de mana e comprar 2 cartas.",
    example:
      "1. Você controla um peão em a3.\n2. Você ativa \"Sacrifício das Massas\".\n3. O peão em a3 é sacrificado e você ganha 6 de mana e compra 2 cartas."
  },
  "archmage-arsenal": {
    name: "Arsenal do Arquimago",
    description:
      "Procure no seu deck por uma carta \"Poder\" com custo de mana 3 ou menos, exceto \"Arsenal do Arquimago\", e adicione-a à sua mão.",
    example:
      "1. Você ativa \"Arsenal do Arquimago\".\n2. Você busca \"Toque do Cavalo\" no deck e adiciona à sua mão."
  },
  "rook-touch": {
    name: "Toque da Torre",
    description:
      "Conceda a qualquer peça que você controla, exceto o rei ou uma torre, a habilidade de se mover como se fosse uma torre por um turno.",
    example: "1. Você tem um cavalo em b2.\n2. Você ativa \"Toque da Torre\".\n3. Você move o cavalo para b7."
  },
  "bishop-touch": {
    name: "Toque do Bispo",
    description:
      "Conceda a qualquer peça que você controla, exceto o rei ou um bispo, a habilidade de se mover como se fosse um bispo por um turno.",
    example: "1. Você tem uma torre em b2.\n2. Você ativa \"Toque do Bispo\".\n3. Você move a torre para c3."
  },
  "retaliate": {
    name: "Retaliação",
    description:
      "Alvo: uma carta no slot de recarga do oponente. Queime x de mana do oponente, sendo x o custo de mana da carta alvo e, se fizer isso, ative essa carta (no seu slot de ignição) para você.",
    example:
      "1. O oponente tem \"Toque do Cavalo\" no slot de recarga.\n2. O oponente ativa qualquer carta \"Poder\".\n3. Você ativa \"Retaliação\" como retribuição.\n4. Você escolhe o \"Toque do Cavalo\" do oponente, queimando 3 de mana do oponente com sucesso.\n5. Você então ativa o \"Toque do Cavalo\" para si, fortalecendo sua torre em a1.\n6. No seu próximo turno você move a torre em a1 para c2."
  },
  "counterattack": {
    name: "Contra-Ataque",
    description:
      "Se uma peça sua seria capturada por uma peça fortalecida por uma carta \"Poder\" neste turno, capture a peça atacante em vez disso.",
    example:
      "1. O oponente ativa \"Toque da Torre\" no peão dele em e4.\n2. Seu oponente tenta capturar sua dama em e6.\n3. Você ativa \"Contra-Ataque\".\n4. O peão atacante é capturado em vez disso."
  },
  "blockade": {
    name: "Bloqueio",
    description:
      "Se uma peça sua seria capturada pela ativação de uma carta \"Contra\" neste turno enquanto ataca, negue o efeito da carta \"Contra\", depois devolva a peça atacante à posição original e escolha outra peça para mover neste turno.",
    example:
      "1. Você move seu peão em e4 que foi fortalecido por \"Toque da Torre\" para e6 na tentativa de capturar a dama do oponente.\n2. O oponente ativa \"Contra-Ataque\" no seu peão.\n3. Você ativa \"Bloqueio\".\n4. O efeito de \"Contra-Ataque\" é negado e seu peão permanece em e4.\n5. Você escolhe outra peça para mover neste turno."
  },
  "backstab": {
    name: "Punhalada",
    description:
      "Alvo: um peão seu que esteja de frente para uma peça do oponente com uma casa vazia atrás dela; salte sobre a peça e capture-a e, se fizer isso, ganhe 3 de mana.",
    example:
      "1. Você ativa \"Punhalada\".\n2. A ignição resolve no turno seguinte.\n3. Você tem um peão em e4.\n4. O oponente tem um cavalo em e5 com uma casa vazia atrás dele.\n5. O peão em e4 salta sobre o cavalo em e5 para e6 e o captura.\n6. Você ganha 3 de mana."
  },
  "stop-right-there": {
    name: "Alto Lá!",
    description: "Alvo: uma carta ignitada do oponente. Negue o efeito dela.",
    example:
      "1. O oponente ativa \"Toque do Cavalo\"\n2. Você ativa \"Alto Lá!\" como retribuição.\n3. O efeito de \"Toque do Cavalo\" é negado."
  },
  "renewal": {
    name: "Renovação",
    description: "Consuma até 10 de mana energizada e ganhe metade desse valor como mana.",
    example:
      "1. Você tem 10 de mana energizada.\n2. Você ativa \"Renovação\".\n3. A ignição resolve no turno seguinte.\n4. Você consome 10 de mana energizada e ganha 5 de mana."
  },
  "life-drain": {
    name: "Dreno de Vida",
    description:
      "Enquanto estiver no slot de ignição, cada captura sua drena 1 de mana do oponente. Quando a ignição desta carta terminar, banir esta carta.",
    example:
      "1. Você ativa \"Dreno de Vida\".\n2. Você captura qualquer peça neste turno.\n3. Você ganha 1 de mana pela captura + 1 de mana do pool de mana do oponente."
  },
  "clairvoyance": {
    name: "Clarividência",
    description:
      "Enquanto estiver no slot de ignição, revele a mão do oponente. Quando a ignição desta carta terminar, banir esta carta.",
    example:
      "1. Você ativa \"Clarividência\".\n2. A mão do oponente é revelada para você e permanece revelada até a ignição desta carta terminar."
  },
  "thunderstorm": {
    name: "Tempestade",
    description:
      "Escolha aleatoriamente 10 casas no tabuleiro; para cada peça sua que esteja numa casa escolhida, ganhe 2 de mana, mas ela não pode mover neste turno; para cada peça que o oponente controle numa casa escolhida, queime 2 de mana do oponente e ela não pode mover no próximo turno de quem a controla.",
    example:
      "1. Você ativa \"Tempestade\".\n2. A ignição resolve no turno seguinte.\n3. Você escolhe aleatoriamente 10 casas no tabuleiro.\n4. 3 peças suas são atingidas: você ganha 6 de mana e elas não podem mover neste turno.\n5. 4 peças que o oponente controla são atingidas: você queima 8 de mana do oponente e essas peças não podem mover no próximo turno delas."
  },
  "extinguish": {
    name: "Extinguir",
    description: "Alvo: uma carta no slot de ignição do oponente. Negue o efeito dela.",
    example:
      "1. O oponente tem \"Turno Duplo\" no slot de ignição.\n2. Você ativa \"Extinguir\".\n3. O efeito de \"Turno Duplo\" é negado e a carta vai para o slot de recarga dele."
  },
  "save-it-for-later": {
    name: "Guardar para Depois",
    description:
      "Esta carta só pode ser ativada se você tiver uma carta no seu slot de ignição. Alvo: essa carta; devolva-a à mão, ganhando mana igual ao custo de mana da carta.",
    example:
      "1. Você tem \"Turno Duplo\" no slot de ignição.\n2. Seu oponente ativa \"Extinguir\" no seu \"Turno Duplo\".\n3. Você ativa \"Guardar para Depois\" como retribuição.\n4. A carta \"Turno Duplo\" volta à sua mão e você ganha 4 de mana."
  }
};

/**
 * Returns catalog rows for createPowerCard for the given locale.
 * @param {string} [locale]
 * @returns {Array<{ type: string, name: string, description: string, example: string, mana: number, ignition: number, cooldown: number }>}
 */
function getLocalizedCardCatalog(locale) {
  const loc = locale === "pt-BR" ? "pt-BR" : "en-US";
  const dict = loc === "pt-BR" ? PT : EN;
  return CARD_ROWS.map((row) => {
    const text = dict[row.id];
    return {
      id: row.id,
      type: row.type,
      name: text.name,
      description: text.description,
      example: text.example,
      mana: row.mana,
      ignition: row.ignition,
      cooldown: row.cooldown
    };
  });
}

globalThis.getLocalizedCardCatalog = getLocalizedCardCatalog;
/** @deprecated Use getLocalizedCardCatalog(locale) */
globalThis.GAME_CARDS_CATALOG = getLocalizedCardCatalog("en-US");
