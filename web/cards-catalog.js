/**
 * Card UI strings: English (name, description, example) and stats come from
 * `__POWER_CHESS_CARD_METADATA__` in `card-metadata.gen.js` (generated from `internal/gameplay/cards.go`).
 * This file holds only **Portuguese** (`PT`) for pt-BR. Regenerate after server catalog edits:
 * `go run ./cmd/export-card-metadata` or `go generate ./internal/gameplay`.
 *
 * PT uses fixed TCG terms: alvo, oponente, mana, ignição, recarga, slot de ignição, slot de recarga,
 * banir, mão, deck, captura (zona de peças capturadas).
 */

/**
 * @typedef {Object} CardCatalogRow
 * @property {string} id
 * @property {string} type
 * @property {number} mana
 * @property {number} ignition
 * @property {number} cooldown
 * @property {number} [targets]
 * @property {number} [effectDuration]
 * @property {string} name
 * @property {string} description
 * @property {string} example
 */

/** @type {CardCatalogRow[]} */
const CARD_ROWS =
  typeof globalThis !== "undefined" && Array.isArray(globalThis.__POWER_CHESS_CARD_METADATA__)
    ? globalThis.__POWER_CHESS_CARD_METADATA__
    : [];

if (typeof globalThis !== "undefined" && CARD_ROWS.length === 0) {
  console.warn(
    "power-chess: load card-metadata.gen.js before cards-catalog.js (run go run ./cmd/export-card-metadata)"
  );
}

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
      "Alvo: uma peça que o oponente controla, exceto o rei ou uma dama, e assuma o controle dela por três turnos. Se a peça for capturada, mova-a para a Captura do oponente.",
    example:
      "1. Você ativa \"Controle Mental\".\n2. A ignição resolve 2 turnos depois.\n3. O oponente controla uma torre em a1.\n4. Você assume o controle da torre em a1 por 3 turnos."
  },
  "zip-line": {
    name: "Tirolesa",
    description:
      "Alvo: uma peça sua, exceto o rei; mova-a para uma casa vazia na mesma fileira (linha).",
    example:
      "1. Você controla um bispo em b2.\n2. Você ativa \"Tirolesa\".\n3. O bispo em b2 move para g2."
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
      "Conceda a qualquer peça que você controla, exceto o rei ou uma torre, a habilidade de se mover como se fosse uma torre por um turno. Se o alvo for um peão, ele só pode mover 1 casa ao usar esse movimento.",
    example: "1. Você tem um cavalo em b2.\n2. Você ativa \"Toque da Torre\".\n3. Você move o cavalo para b7."
  },
  "bishop-touch": {
    name: "Toque do Bispo",
    description:
      "Conceda a qualquer peça que você controla, exceto o rei ou um bispo, a habilidade de se mover como se fosse um bispo por um turno. Se o alvo for um peão, ele só pode mover 1 casa ao usar esse movimento.",
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
      "1. O oponente ativa \"Toque da Torre\" no cavalo dele em e4.\n2. Seu oponente tenta capturar sua dama em e6.\n3. Você ativa \"Contra-Ataque\".\n4. O cavalo atacante é capturado em vez disso."
  },
  "blockade": {
    name: "Bloqueio",
    description:
      "Se uma peça sua seria capturada pela ativação de uma carta \"Contra\" neste turno enquanto ataca, negue o efeito da carta \"Contra\", depois devolva a peça atacante à posição original e escolha outra peça para mover neste turno.",
    example:
      "1. Você move seu cavalo em e4 que foi fortalecido por \"Toque da Torre\" para e6 na tentativa de capturar a dama do oponente.\n2. O oponente ativa \"Contra-Ataque\" no seu cavalo.\n3. Você ativa \"Bloqueio\".\n4. O efeito de \"Contra-Ataque\" é negado e seu cavalo permanece em e4.\n5. Você escolhe outra peça para mover neste turno."
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
      "Escolha aleatoriamente 10 casas no tabuleiro; para cada peça sua que esteja numa casa escolhida, ganhe 2 de mana, mas ela não pode mover neste turno; para cada peça que o oponente controle numa casa escolhida, queime 2 de mana do oponente e ela não pode mover até o fim do próximo turno.",
    example:
      "1. Você ativa \"Tempestade\".\n2. A ignição resolve no turno seguinte.\n3. Você escolhe aleatoriamente 10 casas no tabuleiro.\n4. 3 peças suas são atingidas: você ganha 6 de mana e elas não podem mover neste turno.\n5. 4 peças que o oponente controla são atingidas: você queima 8 de mana do oponente e essas peças não podem mover até o fim do próximo turno."
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
 * @returns {Array<{ type: string, name: string, description: string, example: string, mana: number, ignition: number, cooldown: number, targets?: number, effectDuration?: number }>}
 */
function getLocalizedCardCatalog(locale) {
  const loc = locale === "pt-BR" ? "pt-BR" : "en-US";
  return CARD_ROWS.map((row) => {
    const stats = {
      targets: row.targets ?? 0,
      effectDuration: row.effectDuration ?? 0,
    };
    if (loc === "en-US") {
      return {
        id: row.id,
        type: row.type,
        name: row.name,
        description: row.description,
        example: row.example,
        mana: row.mana,
        ignition: row.ignition,
        cooldown: row.cooldown,
        ...stats,
      };
    }
    const text = PT[row.id];
    if (!text) {
      console.warn("power-chess: missing PT for card id", row.id);
      return {
        id: row.id,
        type: row.type,
        name: row.name,
        description: row.description,
        example: row.example,
        mana: row.mana,
        ignition: row.ignition,
        cooldown: row.cooldown,
        ...stats,
      };
    }
    return {
      id: row.id,
      type: row.type,
      name: text.name,
      description: text.description,
      example: text.example,
      mana: row.mana,
      ignition: row.ignition,
      cooldown: row.cooldown,
      ...stats,
    };
  });
}

/**
 * Display/deck sort order: Power > Continuous > Retribution > Counter.
 * @type {Record<string, number>}
 */
const CARD_TYPE_SORT_ORDER = {
  power: 0,
  continuous: 1,
  retribution: 2,
  counter: 3
};

/**
 * Sort rank for a card type string (lower = earlier). Unknown types sort last.
 * @param {string} [type]
 * @returns {number}
 */
function cardTypeSortRank(type) {
  if (!type || typeof type !== "string") return 999;
  const k = type.toLowerCase();
  const rank = CARD_TYPE_SORT_ORDER[k];
  return rank !== undefined ? rank : 999;
}

/**
 * Compare two catalog rows by type order, then name.
 * @param {{ type?: string, name?: string }} a
 * @param {{ type?: string, name?: string }} b
 * @returns {number}
 */
function compareCatalogRowsByTypeThenName(a, b) {
  const ra = cardTypeSortRank(a?.type);
  const rb = cardTypeSortRank(b?.type);
  if (ra !== rb) return ra - rb;
  return String(a?.name || "").localeCompare(String(b?.name || ""));
}

/**
 * Compare two card ids using localized catalog rows (type, then name).
 * @param {string} idA
 * @param {string} idB
 * @param {Map<string, { type?: string, name?: string }>} byId
 * @returns {number}
 */
function compareCardIdsByTypeThenName(idA, idB, byId) {
  const a = byId.get(idA);
  const b = byId.get(idB);
  return compareCatalogRowsByTypeThenName(
    a || { name: idA },
    b || { name: idB }
  );
}

globalThis.getLocalizedCardCatalog = getLocalizedCardCatalog;
globalThis.cardTypeSortRank = cardTypeSortRank;
globalThis.compareCatalogRowsByTypeThenName = compareCatalogRowsByTypeThenName;
globalThis.compareCardIdsByTypeThenName = compareCardIdsByTypeThenName;
/** @deprecated Use getLocalizedCardCatalog(locale) */
globalThis.GAME_CARDS_CATALOG = getLocalizedCardCatalog("en-US");
