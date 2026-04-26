import { PLAYMAT_UI_TEST_OVERLAY } from "/ui-test-config.js";
import {
  matchTestAutoApplyEnabled,
  matchTestAutoConfirmMulliganEnabled,
  buildMatchDebugFixturePayload,
} from "/match-test-config.js";

(function () {
  let ws = null;
  let seq = 1;
  let lastSnapshot = null;
  let selectedFrom = null;
  /**
   * Two-step targeting: after `ignite_card` (hand→ignition), server sets `ignitionTargeting.awaitingTargetChoice`;
   * then the player picks a square and sends `submit_ignition_targets`.
   * @type {{ stage: "placed" | "picking", cardId: string } | null}
   */
  let igniteTargetFlow = null;
  /**
   * Two-step flow for Disruption cards played during an ignite_reaction window.
   * Step 1: user selects the Disruption card → stored here, banish selection mode activates.
   * Step 2: user selects a Power card from hand → queue_reaction sent with both handIndex and banishHandIndex.
   * Null when not in banish selection mode.
   * @type {{ handIndex: number } | null}
   */
  let disruptionBanishFlow = null;
  /**
   * Two-step flow for Retaliate reactions.
   * Step 1: user selects Retaliate from hand.
   * Step 2: user selects a valid Power card from the opponent cooldown zone.
   * @type {{ handIndex: number } | null}
   */
  let retaliateTargetFlow = null;
  /** @type {{ row: number, col: number } | null} */
  let copiedZipLineSource = null;
  /**
   * Keeps blue dotted targets until the matching `activate_card` is processed (success or fail).
   * @type {{ owner: string, cardId: string, pieces: { row: number, col: number }[] } | null}
   */
  let ignitionBlueHold = null;
  /** After clicking/dragging a board piece to move, suppress enchanted-piece card hover until mouse leaves the square. */
  let boardEnchantHoverSuppressed = false;
  let highlightedMoves = [];
  let draggingFrom = null;
  /** When true, block outgoing game actions during turn-start resource animation. */
  let turnResourceAnimBlocking = false;
  let effectAnimBlocking = false;
  let effectAnimBlockTimeout = null;
  /** Wall-clock ms when effect-anim input block ends (extends on overlapping blocks). */
  let effectAnimUnblockAt = 0;
  /**
   * `activate_card` payloads received before the matching `state_snapshot` is applied.
   * Flushed after the "resolving effects" banner when the reaction stack had 2+ cards, or earlier otherwise.
   */
  /** @type {object[]} */
  let pendingActivateCardPayloads = [];
  /** Server `activate_card` events processed in order (reaction pile + ignition finalize). */
  /** @type {object[]} */
  let activationFxQueue = [];
  /** @type {Promise<void> | null} */
  let activationFxWorkerPromise = null;
  /** @type {boolean} */
  let gameStarted = false;
  let joinedRoom = false;
  let roomListTimer = null;
  let prevMatchEnded = false;
  let lobbyRooms = [];
  let revealRoomPassword = false;
  let matchCountdownTimer = null;
  let pendingJoinAttempt = null;
  /** @type {string} Room name shown in the private-join modal (for i18n refresh). */
  let privateJoinPendingRoomName = "";
  /** @type {object[]} Recent WebSocket traffic and UI events (export + optional server mirror). */
  let clientTraceBuffer = [];
  /** @type {ReturnType<typeof setInterval> | null} */
  let clientTraceMirrorTimerId = null;
  const CLIENT_TRACE_CAP = 600;

  /** @type {Set<number>} Hand indices selected to return to the deck during mulligan (local UI only). */
  let mulliganPick = new Set();
  /** Updates mulligan countdown text while the opening phase is active. */
  let mulliganUiTimerId = null;
  /** @type {((value: string | null) => void) | null} Resolves the open promotion modal choice. */
  let promotionChoiceResolver = null;
  /**
   * Tracks whether a deck-search pending effect modal is currently open for the local player.
   * Set to the card ID of the pending effect when open, null otherwise.
   * @type {string | null}
   */
  let deckSearchOpenForCard = null;

  /** Set after we send debug_match_fixture (match-test-config); reset on new WebSocket. */
  let matchTestFixtureSent = false;
  /** Previous snapshot had both seats connected (for detecting second player join). */
  let matchTestPrevBothConnected = false;
  /** After fixture sent with auto-confirm: confirm mulligan on next applicable snapshot. */
  let matchTestAwaitingMulliganConfirm = false;

  const AUTH_TOKEN_KEY = "powerChessAuthToken";
  /** When false, server has no DATABASE_URL auth; lobby works without login. */
  let authBackendAvailable = true;
  /** @type {{ id: number, username: string, email: string, role: string } | null} */
  let authUser = null;

  const lobbyScreenEl = document.getElementById("lobbyScreen");
  const gameShellEl = document.getElementById("gameShell");
  const waitingBannerEl = document.getElementById("waitingBanner");
  const opponentDisconnectOverlayEl = document.getElementById("opponentDisconnectOverlay");
  const boardAreaEl = document.getElementById("boardArea");
  const roomListEl = document.getElementById("roomList");
  const roomListEmptyEl = document.getElementById("roomListEmpty");
  const localeSelectEl = document.getElementById("localeSelect");
  const inRoomLabelEl = document.getElementById("inRoomLabel");
  const roomNameEl = document.getElementById("roomName");
  const roomSearchEl = document.getElementById("roomSearch");
  const privateRoomEl = document.getElementById("privateRoom");
  const roomPasswordFieldEl = document.getElementById("roomPasswordField");
  const roomPasswordEl = document.getElementById("roomPassword");
  const roomPasswordToggleEl = document.getElementById("roomPasswordToggle");
  const lobbyPrivatePasswordErrorEl = document.getElementById("lobbyPrivatePasswordError");
  const pieceTypeEl = document.getElementById("pieceType");
  const boardWrapEl = document.getElementById("boardWrap");
  const boardFrameEl = document.getElementById("boardFrame");
  const captureThreatOverlayEl = document.getElementById("captureThreatOverlay");
  const snapshotEl = document.getElementById("snapshot");
  const eventsEl = document.getElementById("events");
  const exportClientTraceBtnEl = document.getElementById("exportClientTraceBtn");
  const statusEl = document.getElementById("status");

  const playerEl = document.getElementById("playerId");
  const reactionModeSelectEl = document.getElementById("reactionModeSelect");
  const reactionModeLabelEl = document.getElementById("reactionModeLabel");
  const reactionPassBtnEl = document.getElementById("reactionPassBtn");
  const coordsInSquaresEl = document.getElementById("coordsInSquares");
  const coordsInSquaresTextEl = document.getElementById("coordsInSquaresText");
  const effectTurnsAlwaysVisibleEl = document.getElementById("effectTurnsAlwaysVisible");
  const effectTurnsToggleTextEl = document.getElementById("effectTurnsToggleText");
  const manaFillA = document.getElementById("manaFillA");
  const manaFillB = document.getElementById("manaFillB");
  const manaLabelA = document.getElementById("manaLabelA");
  const manaLabelB = document.getElementById("manaLabelB");
  const energizedFillA = document.getElementById("energizedFillA");
  const energizedFillB = document.getElementById("energizedFillB");
  const energizedLabelA = document.getElementById("energizedLabelA");
  const energizedLabelB = document.getElementById("energizedLabelB");
  const matchEndOverlayEl = document.getElementById("matchEndOverlay");
  const matchEndBodyEl = document.getElementById("matchEndBody");
  const matchEndCountdownEl = document.getElementById("matchEndCountdown");
  const matchEndRematchEl = document.getElementById("matchEndRematch");
  const matchEndStayEl = document.getElementById("matchEndStay");
  const matchEndToLobbyEl = document.getElementById("matchEndToLobby");
  const authOverlayEl = document.getElementById("authOverlay");
  const authUnavailableHintEl = document.getElementById("authUnavailableHint");
  const authRegUsernameEl = document.getElementById("authRegUsername");
  const authRegEmailEl = document.getElementById("authRegEmail");
  const authRegPasswordEl = document.getElementById("authRegPassword");
  const authRegConfirmEl = document.getElementById("authRegConfirm");
  const authRegisterBtnEl = document.getElementById("authRegisterBtn");
  const authLoginEmailEl = document.getElementById("authLoginEmail");
  const authLoginPasswordEl = document.getElementById("authLoginPassword");
  const authLoginBtnEl = document.getElementById("authLoginBtn");
  const authErrorEl = document.getElementById("authError");
  const lobbyUserLabelEl = document.getElementById("lobbyUserLabel");
  const logoutBtnEl = document.getElementById("logoutBtn");
  const privateJoinOverlayEl = document.getElementById("privateJoinOverlay");
  const privateJoinTitleEl = document.getElementById("privateJoinTitle");
  const privateJoinBodyEl = document.getElementById("privateJoinBody");
  const privateJoinPasswordLabelEl = document.getElementById("privateJoinPasswordLabel");
  const privateJoinPasswordInputEl = document.getElementById("privateJoinPasswordInput");
  const privateJoinPasswordToggleEl = document.getElementById("privateJoinPasswordToggle");
  const privateJoinCancelEl = document.getElementById("privateJoinCancel");
  const privateJoinSubmitEl = document.getElementById("privateJoinSubmit");
  const privateJoinErrorEl = document.getElementById("privateJoinError");
  const cardMarqueeLabelEl = document.getElementById("cardMarqueeLabel");
  const mainFooterEl = document.getElementById("mainFooter");
  const lobbyDeckRowEl = document.getElementById("lobbyDeckRow");
  const lobbyDeckSelectEl = document.getElementById("lobbyDeckSelect");
  const lobbyDeckHintEl = document.getElementById("lobbyDeckHint");
  const lobbyDeckAlertEl = document.getElementById("lobbyDeckAlert");
  const lobbyDeckViewBtnEl = document.getElementById("lobbyDeckViewBtn");
  const lobbyDeckBuilderLinkEl = document.getElementById("lobbyDeckBuilderLink");
  const deckViewModalEl = document.getElementById("deckViewModal");
  const deckViewGridEl = document.getElementById("deckViewGrid");
  const deckViewTitleEl = document.getElementById("deckViewTitle");
  const deckViewCloseBtnEl = document.getElementById("deckViewCloseBtn");
  const promotionOverlayEl = document.getElementById("promotionOverlay");
  const promotionTitleEl = document.getElementById("promotionTitle");
  const promotionBodyEl = document.getElementById("promotionBody");
  const promotionChoicesEl = document.getElementById("promotionChoices");
  const promotionCancelEl = document.getElementById("promotionCancel");
  const deckSearchOverlayEl = document.getElementById("deckSearchOverlay");
  const deckSearchGridEl = document.getElementById("deckSearchGrid");
  const deckSearchEmptyEl = document.getElementById("deckSearchEmpty");
  const deckSearchConfirmRowEl = document.getElementById("deckSearchConfirmRow");
  const deckSearchConfirmBtnEl = document.getElementById("deckSearchConfirmBtn");

  const i18n = {
    "en-US": {
      title: "POWER CHESS (Alpha)",
      language: "Language",
      roomName: "Room Name",
      pieceType: "Piece Type",
      pieceTypeRandom: "Random",
      pieceTypeWhite: "White",
      pieceTypeBlack: "Black",
      create: "Create",
      privateRoom: "Private Room",
      password: "Password",
      passwordPlaceholder: "Room password",
      hint: "Choose the room name and piece side you want to play, then click Create. Join an existing room by clicking it in the room list. The match starts when both players are in the room.",
      openRooms: "Open Rooms",
      searchLabel: "Search (ID or Name)",
      searchPlaceholder: "Search rooms...",
      noRooms: "No active rooms right now.",
      waiting: "Waiting for opponent...",
      matchFinished: "Match finished",
      backLobby: "Back to lobby",
      playAgain: "Play again",
      stayInRoom: "Stay in room",
      leaveRoom: "Leave room",
      copyRoomId: "Copy ID",
      idCopied: "ID copied!",
      privateJoinTitle: "Private room",
      privateJoinDescription: 'Enter the password to join "{name}".',
      joinRoom: "Join",
      cancel: "Cancel",
      privateNeedsPassword: "Enter the private room password.",
      privateJoinNeedsPassword: "This room is private. Enter the room password.",
      private: "Private",
      public: "Public",
      room: "Room",
      you: "You",
      player: "Player",
      passwordLabelInline: "Password",
      show: "show",
      hide: "hide",
      statusWaiting: "waiting",
      statusPlaying: "in game",
      reactions: "Reactions",
      reactionModeOff: "Off",
      reactionModeOn: "On",
      reactionModeAuto: "Auto",
      reactionPassOk: "Confirm Play",
      toggleOn: "On",
      toggleOff: "Off",
      coordsLabel: "Coords",
      coordsInSquares: "Coords in squares",
      effectTurnsLabel: "Effect turns",
      effectTurnsAlways: "Always",
      effectTurnsHover: "Hover",
      submitMove: "Submit move",
      activateCard: "Activate card",
      resolvePending: "Resolve pending effect",
      queueReaction: "Queue reaction",
      resolveReactions: "Resolve reactions",
      resolvingEffects: "Resolving Effects",
      reactionPendingTitle: "Reaction/Pending status",
      snapshotTitle: "Snapshot",
      eventsTitle: "Events",
      exportClientTrace: "Download client trace (JSON)",
      fromPlaceholder: "from row,col",
      toPlaceholder: "to row,col",
      handIndexPlaceholder: "hand index",
      pendingPiecePlaceholder: "pending piece row,col",
      reactionHandIndexPlaceholder: "reaction hand index",
      reactionPiecePlaceholder: "reaction target row,col (optional)",
      reasonCheckmate: "Reason: checkmate.",
      reasonStalemate: "Reason: stalemate.",
      reasonBothDisconnected: "Reason: both players disconnected (match canceled).",
      reasonDisconnectTimeout: "Reason: opponent disconnected (timeout).",
      reasonLeftRoom: "Reason: opponent left the room.",
      youWon: "You won!",
      youLost: "You lost!",
      draw: "Draw!",
      matchEndedNoWinner: "Match ended.",
      reasonPrefix: "Reason",
      reasonOpponentAbandoned: "Reason: Your opponent abandoned the match.",
      reasonCheckmateShort: "Reason: checkmate.",
      reasonStalemateShort: "Reason: stalemate.",
      disconnectWinAlert: "Victory: opponent disconnected and did not return in time.",
      opponentDisconnectedBanner: "Opponent disconnected ({s}s)",
      opponentDisconnectedHint:
        "You will win when the timer reaches 0 if they do not return. This seat is closed to other players until then.",
      rematchProposed: "New game proposed, click on 'Play again' to accept.",
      rematchWaiting: "Waiting for opponent to accept the new game.",
      rematchOpponentLeft: "The other player left the room.",
      autoCloseIn: "Room closes in {s}s if no action is taken.",
      autoCloseNow: "Room will close now if no action is taken.",
      cardMarqueeTitle: "All cards — layout preview",
      authCreateTitle: "Create account",
      authAlreadyHave: "Already have an account?",
      authUsername: "Username",
      authEmail: "Email",
      authPassword: "Password",
      authConfirmPassword: "Confirm password",
      authRegister: "Create account",
      authLogin: "Log in",
      authLogout: "Log out",
      authErrorMismatch: "Passwords do not match.",
      authErrorShort: "Password must be at least 8 characters.",
      authErrorNetwork: "Could not reach the server.",
      authErrorTaken: "Username or email already in use.",
      authErrorInvalid: "Invalid email or password.",
      authErrorGeneric: "Something went wrong. Try again.",
      authUnavailable: "Accounts are disabled on this server (no database). You can still play as a guest.",
      lobbySignedInAs: "Signed in as {username}",
      lobbyGuest: "Guest (no account)",
      lobbyDeckLabel: "Deck for match",
      lobbyDeckHint: "This deck is used when you join or create a room. Change it before connecting.",
      noSavedDeckAlert: "You have no saved deck. Use Deck Builder to create one (20 cards) before playing.",
      lobbyDeckView: "View",
      lobbyDeckBuilder: "Deck Builder",
      deckViewClose: "Close",
      debugLogsTitle: "Debug logs",
      zoneHand: "Hand",
      zoneDeck: "Deck",
      zoneIgnition: "Ignition",
      zoneCooldown: "Cooldown",
      zoneBanished: "Banished",
      zoneCapture: "Capture",
      zoneCaptureAriaYour: "Your captures",
      zoneCaptureAriaOpponent: "Opponent captures",
      drawFromDeck: "DRAW",
      connectErrorPrefix: "Could not connect:",
      mulliganHint: "Tap cards to mark them red (they return to the deck). Confirm when ready.",
      disruptionBanishHint: "Select a Power card from your hand to banish as the reaction cost. Press Esc to cancel.",
      retaliateTargetHint: "Select a valid Power card from your opponent's cooldown zone. Press Esc to cancel.",
      selectIgnitionTargetHint: "Select the card target on the board.",
      selectOwnPieceHint: "Select one of your pieces on the board.",
      selectOwnPawnHint: "Select one of your pawns to sacrifice.",
      selectOpponentPieceHint: "Select an opponent piece on the board.",
      selectPieceSwapFirstHint: "Select one of your pieces to swap.",
      selectPieceSwapSecondHint: "Select an opponent piece within 2 squares.",
      selectZipLineDestinationHint: "Select an empty square in the same row.",
      selectDeckSearchCardHint: "Select a card from your deck to add to your hand.",
      confirmPlayHint: "Confirm to continue the play.",
      mulliganConfirm: "Confirm mulligan",
      mulliganWaitingYou: "Waiting for you to confirm…",
      mulliganWaitingOpp: "Waiting for opponent…",
      mulliganLine: "Mulligan — White: {w} | Black: {b}",
      mulliganPending: "…",
      mulliganAutoIn: "Auto-confirms in {s}s",
      promotionTitle: "Choose promotion",
      promotionBody: "Choose the piece your pawn promotes to.",
      promotionQueen: "Queen",
      promotionRook: "Rook",
      promotionBishop: "Bishop",
      promotionKnight: "Knight",
    },
    "pt-BR": {
      title: "POWER CHESS (Alpha)",
      language: "Idioma",
      roomName: "Nome da sala",
      pieceType: "Tipo de peça",
      pieceTypeRandom: "Aleatório",
      pieceTypeWhite: "Brancas",
      pieceTypeBlack: "Pretas",
      create: "Criar",
      privateRoom: "Sala privada",
      password: "Senha",
      passwordPlaceholder: "Senha da sala",
      hint: "Escolha o nome da sala e o lado que deseja jogar, depois clique em Criar. Entre em uma sala existente clicando nela na lista. A partida irá começar quando ambos jogadores estiverem na sala.",
      openRooms: "Salas abertas",
      searchLabel: "Buscar (ID ou nome)",
      searchPlaceholder: "Buscar salas...",
      noRooms: "Nenhuma sala ativa no momento.",
      waiting: "Aguardando oponente...",
      matchFinished: "Partida encerrada",
      backLobby: "Voltar ao lobby",
      playAgain: "Jogar novamente",
      stayInRoom: "Ficar na sala",
      leaveRoom: "Sair da sala",
      copyRoomId: "Copiar ID",
      idCopied: "ID copiado!",
      privateJoinTitle: "Sala privada",
      privateJoinDescription: 'Digite a senha para entrar em "{name}".',
      joinRoom: "Entrar",
      cancel: "Cancelar",
      privateNeedsPassword: "Informe a senha da sala privada.",
      privateJoinNeedsPassword: "Esta sala é privada. Informe a senha da sala.",
      private: "Privada",
      public: "Pública",
      room: "Sala",
      you: "Você",
      player: "Player",
      passwordLabelInline: "Senha",
      show: "mostrar",
      hide: "ocultar",
      statusWaiting: "aguardando",
      statusPlaying: "em partida",
      reactions: "Reações",
      reactionModeOff: "Desligado",
      reactionModeOn: "Ligado",
      reactionModeAuto: "Automático",
      reactionPassOk: "Confirmar Jogada",
      toggleOn: "Ligado",
      toggleOff: "Desligado",
      coordsLabel: "Coords",
      coordsInSquares: "Coordenadas nas casas",
      effectTurnsLabel: "Turnos do efeito",
      effectTurnsAlways: "Sempre",
      effectTurnsHover: "Hover",
      submitMove: "Enviar jogada",
      activateCard: "Ativar carta",
      resolvePending: "Resolver efeito pendente",
      queueReaction: "Enfileirar reação",
      resolveReactions: "Resolver reações",
      resolvingEffects: "Resolvendo Efeitos",
      reactionPendingTitle: "Status de reação/pendente",
      snapshotTitle: "Snapshot",
      eventsTitle: "Eventos",
      exportClientTrace: "Baixar trace do cliente (JSON)",
      fromPlaceholder: "origem linha,col",
      toPlaceholder: "destino linha,col",
      handIndexPlaceholder: "índice da mão",
      pendingPiecePlaceholder: "peça pendente linha,col",
      reactionHandIndexPlaceholder: "índice reação",
      reactionPiecePlaceholder: "alvo da reação linha,col (opcional)",
      reasonCheckmate: "Motivo: xeque-mate.",
      reasonStalemate: "Motivo: empate por afogamento.",
      reasonBothDisconnected: "Motivo: ambos desconectaram (partida cancelada).",
      reasonDisconnectTimeout: "Motivo: vitória por desconexão do oponente.",
      reasonLeftRoom: "Motivo: vitória por saída do oponente da sala.",
      youWon: "Você venceu!",
      youLost: "Você perdeu!",
      draw: "Empate!",
      matchEndedNoWinner: "Partida encerrada.",
      reasonPrefix: "Motivo",
      reasonOpponentAbandoned: "Motivo: o oponente abandonou a partida.",
      reasonCheckmateShort: "Motivo: Xeque-mate.",
      reasonStalemateShort: "Motivo: Afogamento (stalemate).",
      disconnectWinAlert: "Vitória: o oponente não voltou a tempo (desconexão).",
      opponentDisconnectedBanner: "Jogador desconectado ({s}s)",
      opponentDisconnectedHint:
        "Você vence quando o tempo chegar a 0 se o oponente não voltar. Este assento permanece fechado a novas entradas até lá.",
      rematchProposed: "Novo jogo proposto, clique em 'Jogar novamente' para aceitar.",
      rematchWaiting: "Aguardando o oponente aceitar o novo jogo.",
      rematchOpponentLeft: "O oponente saiu da sala.",
      autoCloseIn: "A sala fecha em {s}s se ninguém fizer nada.",
      autoCloseNow: "A sala será fechada agora se ninguém fizer nada.",
      cardMarqueeTitle: "Todas as cartas — prévia do layout",
      authCreateTitle: "Criar conta",
      authAlreadyHave: "Já possui conta?",
      authUsername: "Nome de usuário",
      authEmail: "E-mail",
      authPassword: "Senha",
      authConfirmPassword: "Confirmar senha",
      authRegister: "Criar conta",
      authLogin: "Entrar",
      authLogout: "Sair",
      authErrorMismatch: "As senhas não coincidem.",
      authErrorShort: "A senha deve ter pelo menos 8 caracteres.",
      authErrorNetwork: "Não foi possível contatar o servidor.",
      authErrorTaken: "Nome de usuário ou e-mail já em uso.",
      authErrorInvalid: "E-mail ou senha inválidos.",
      authErrorGeneric: "Algo deu errado. Tente novamente.",
      authUnavailable: "Contas desativadas neste servidor (sem banco). Você ainda pode jogar como convidado.",
      lobbySignedInAs: "Conectado como {username}",
      lobbyGuest: "Convidado (sem conta)",
      lobbyDeckLabel: "Deck para a partida",
      lobbyDeckHint: "Este deck é usado ao criar ou entrar em uma sala. Altere antes de conectar.",
      noSavedDeckAlert:
        "Você não tem nenhum deck salvo. Use o Editor de deck para criar um (20 cartas) antes de jogar.",
      lobbyDeckView: "Visualizar",
      lobbyDeckBuilder: "Editor de deck",
      deckViewClose: "Fechar",
      debugLogsTitle: "Logs de debug",
      zoneHand: "Mão",
      zoneDeck: "Deck",
      zoneIgnition: "Ignição",
      zoneCooldown: "Recarga",
      zoneBanished: "Banidas",
      zoneCapture: "Captura",
      zoneCaptureAriaYour: "Suas capturas",
      zoneCaptureAriaOpponent: "Capturas do oponente",
      drawFromDeck: "Comprar",
      connectErrorPrefix: "Não foi possível conectar:",
      mulliganHint: "Toque nas cartas para marcar em vermelho (voltam ao deck). Confirme quando terminar.",
      disruptionBanishHint:
        "Selecione uma carta Power da mão para banir como custo de reação. Pressione Esc para cancelar.",
      retaliateTargetHint:
        "Selecione uma carta Power válida na recarga do oponente. Pressione Esc para cancelar.",
      selectIgnitionTargetHint: "Selecione o alvo da carta no tabuleiro.",
      selectOwnPieceHint: "Selecione uma das suas peças no tabuleiro.",
      selectOwnPawnHint: "Selecione um dos seus peões para sacrificar.",
      selectOpponentPieceHint: "Selecione uma peça do oponente no tabuleiro.",
      selectPieceSwapFirstHint: "Selecione uma das suas peças para trocar.",
      selectPieceSwapSecondHint: "Selecione uma peça do oponente a até 2 casas.",
      selectZipLineDestinationHint: "Selecione uma casa vazia na mesma linha.",
      selectDeckSearchCardHint: "Selecione uma carta do seu deck para adicionar à mão.",
      confirmPlayHint: "Confirme para continuar a jogada.",
      mulliganConfirm: "Confirmar mulligan",
      mulliganWaitingYou: "Aguardando sua confirmação…",
      mulliganWaitingOpp: "Aguardando o oponente…",
      mulliganLine: "Mulligan — Brancas: {w} | Pretas: {b}",
      mulliganPending: "…",
      mulliganAutoIn: "Confirmação automática em {s}s",
      promotionTitle: "Escolha a promoção",
      promotionBody: "Escolha a peça para promover o peão.",
      promotionQueen: "Rainha",
      promotionRook: "Torre",
      promotionBishop: "Bispo",
      promotionKnight: "Cavalo",
    },
  };
  let locale = "en-US";

  function t(key, vars = {}) {
    const dict = i18n[locale] || i18n["en-US"];
    const fallback = i18n["en-US"][key] || key;
    let str = dict[key] || fallback;
    for (const [k, v] of Object.entries(vars)) {
      str = str.replaceAll(`{${k}}`, String(v));
    }
    return str;
  }

  function readStoredToken() {
    try {
      return localStorage.getItem(AUTH_TOKEN_KEY) || "";
    } catch (_) {
      return "";
    }
  }

  function writeStoredToken(tok) {
    try {
      if (tok) localStorage.setItem(AUTH_TOKEN_KEY, tok);
      else localStorage.removeItem(AUTH_TOKEN_KEY);
    } catch (_) {
      /* ignore */
    }
  }

  function authFetchHeaders() {
    const tok = readStoredToken();
    return tok ? { Authorization: `Bearer ${tok}` } : {};
  }

  function setAuthErrorVisible(msg) {
    if (!authErrorEl) return;
    if (!msg) {
      authErrorEl.classList.add("hidden");
      authErrorEl.textContent = "";
      return;
    }
    authErrorEl.textContent = msg;
    authErrorEl.classList.remove("hidden");
  }

  function refreshLobbyUserLabel() {
    if (!lobbyUserLabelEl) return;
    if (!authBackendAvailable) {
      lobbyUserLabelEl.textContent = t("lobbyGuest");
      logoutBtnEl.classList.add("hidden");
      void refreshLobbyDecks();
      return;
    }
    if (authUser) {
      lobbyUserLabelEl.textContent = t("lobbySignedInAs", { username: authUser.username });
      logoutBtnEl.classList.remove("hidden");
    } else {
      lobbyUserLabelEl.textContent = "";
      logoutBtnEl.classList.add("hidden");
    }
  }

  function showAuthOverlay() {
    if (!authBackendAvailable || !authOverlayEl) return;
    authOverlayEl.classList.remove("hidden");
    authOverlayEl.setAttribute("aria-hidden", "false");
  }

  function hideAuthOverlay() {
    if (!authOverlayEl) return;
    authOverlayEl.classList.add("hidden");
    authOverlayEl.setAttribute("aria-hidden", "true");
    setAuthErrorVisible("");
  }

  async function authResponseErrorMessage(r, fallbackKey) {
    try {
      const data = await r.json();
      if (data && typeof data.error === "string") return data.error;
    } catch (_) {
      /* ignore */
    }
    return t(fallbackKey);
  }

  async function applySessionFromMeResponse(r) {
    if (r.status === 503) {
      authBackendAvailable = false;
      authUser = null;
      writeStoredToken("");
      hideAuthOverlay();
      if (authUnavailableHintEl) authUnavailableHintEl.classList.add("hidden");
      refreshLobbyUserLabel();
      return;
    }
    authBackendAvailable = true;
    if (r.ok) {
      authUser = await r.json();
      hideAuthOverlay();
      if (authUnavailableHintEl) authUnavailableHintEl.classList.add("hidden");
      refreshLobbyUserLabel();
      await refreshLobbyDecks();
      return;
    }
    authUser = null;
    writeStoredToken("");
    if (r.status === 401) {
      showAuthOverlay();
      refreshLobbyUserLabel();
      await refreshLobbyDecks();
      return;
    }
    hideAuthOverlay();
    refreshLobbyUserLabel();
    await refreshLobbyDecks();
  }

  async function bootstrapAuthSession() {
    try {
      const r = await fetch("/api/auth/me", { headers: authFetchHeaders() });
      await applySessionFromMeResponse(r);
    } catch (_) {
      authBackendAvailable = true;
      setAuthErrorVisible(t("authErrorNetwork"));
      showAuthOverlay();
    }
  }

  async function submitRegister() {
    setAuthErrorVisible("");
    const username = (authRegUsernameEl.value || "").trim();
    const email = (authRegEmailEl.value || "").trim();
    const password = authRegPasswordEl.value || "";
    const confirm = authRegConfirmEl.value || "";
    if (password !== confirm) {
      setAuthErrorVisible(t("authErrorMismatch"));
      return;
    }
    if (password.length < 8) {
      setAuthErrorVisible(t("authErrorShort"));
      return;
    }
    try {
      const r = await fetch("/api/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username,
          email,
          password,
          confirm_password: confirm,
        }),
      });
      if (r.status === 503) {
        authUnavailableHintEl.textContent = t("authUnavailable");
        authUnavailableHintEl.classList.remove("hidden");
        await applySessionFromMeResponse(r);
        return;
      }
      if (!r.ok) {
        const msg =
          r.status === 409
            ? t("authErrorTaken")
            : r.status === 400
              ? await authResponseErrorMessage(r, "authErrorGeneric")
              : await authResponseErrorMessage(r, "authErrorGeneric");
        setAuthErrorVisible(msg);
        return;
      }
      const data = await r.json();
      writeStoredToken(data.token || "");
      authUser = data.user || null;
      hideAuthOverlay();
      refreshLobbyUserLabel();
      await refreshLobbyDecks();
    } catch (_) {
      setAuthErrorVisible(t("authErrorNetwork"));
    }
  }

  async function submitLogin() {
    setAuthErrorVisible("");
    const email = (authLoginEmailEl.value || "").trim();
    const password = authLoginPasswordEl.value || "";
    try {
      const r = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      if (r.status === 503) {
        authUnavailableHintEl.textContent = t("authUnavailable");
        authUnavailableHintEl.classList.remove("hidden");
        await applySessionFromMeResponse(r);
        return;
      }
      if (!r.ok) {
        const msg = r.status === 401 ? t("authErrorInvalid") : await authResponseErrorMessage(r, "authErrorGeneric");
        setAuthErrorVisible(msg);
        return;
      }
      const data = await r.json();
      writeStoredToken(data.token || "");
      authUser = data.user || null;
      hideAuthOverlay();
      refreshLobbyUserLabel();
      await refreshLobbyDecks();
    } catch (_) {
      setAuthErrorVisible(t("authErrorNetwork"));
    }
  }

  function logoutSession() {
    writeStoredToken("");
    authUser = null;
    if (authBackendAvailable) showAuthOverlay();
    refreshLobbyUserLabel();
    if (authLoginEmailEl) authLoginEmailEl.value = "";
    if (authLoginPasswordEl) authLoginPasswordEl.value = "";
    void refreshLobbyDecks();
  }

  async function refreshLobbyDecks() {
    if (!lobbyDeckRowEl) return;
    lobbyDeckRowEl.classList.add("hidden");
    lobbyDeckAlertEl.classList.add("hidden");
    lobbyDeckHintEl.textContent = "";
    lobbyDeckSelectEl.innerHTML = "";
    if (!authBackendAvailable || !authUser || !readStoredToken()) {
      return;
    }
    try {
      const r = await fetch("/api/decks", { headers: { ...authFetchHeaders(), Accept: "application/json" } });
      if (r.status === 503 || r.status === 401) return;
      if (!r.ok) return;
      const data = await r.json();
      const decks = data.decks || [];
      const lobbyId = data.lobbyDeckId;
      for (const d of decks) {
        const opt = document.createElement("option");
        opt.value = String(d.id);
        opt.textContent = d.name;
        lobbyDeckSelectEl.appendChild(opt);
      }
      if (lobbyId != null && lobbyDeckSelectEl.querySelector(`option[value="${lobbyId}"]`)) {
        lobbyDeckSelectEl.value = String(lobbyId);
      } else if (decks[0]) {
        lobbyDeckSelectEl.value = String(decks[0].id);
      }
      lobbyDeckHintEl.textContent = t("lobbyDeckHint");
      lobbyDeckRowEl.classList.remove("hidden");
      if (decks.length === 0) {
        lobbyDeckAlertEl.textContent = t("noSavedDeckAlert");
        lobbyDeckAlertEl.classList.remove("hidden");
      }
    } catch (_) {
      /* ignore */
    }
  }

  function closeDeckViewModal() {
    deckViewModalEl.classList.add("hidden");
    deckViewModalEl.setAttribute("aria-hidden", "true");
    deckViewGridEl.innerHTML = "";
  }

  /**
   * Opens the in-match deck-search modal for a pending deck-search effect.
   * @param {Array<{cardId: string}>} choices - Eligible cards from the server.
   * @param {string} pendingCardID - The card ID of the pending effect (e.g. "archmage-arsenal").
   */
  function openDeckSearchModal(choices, pendingCardID) {
    if (!deckSearchOverlayEl || !deckSearchGridEl) return;
    deckSearchOpenForCard = pendingCardID;
    deckSearchGridEl.innerHTML = "";

    const hasChoices = Array.isArray(choices) && choices.length > 0;
    if (deckSearchEmptyEl) deckSearchEmptyEl.classList.toggle("hidden", hasChoices);
    if (deckSearchConfirmRowEl) deckSearchConfirmRowEl.classList.toggle("hidden", hasChoices);

    if (hasChoices) {
      const catalog = getLocalizedCardCatalog(locale);
      const byId = new Map(catalog.map((c) => [c.id, c]));
      for (const choice of choices) {
        const def = byId.get(choice.cardId);
        if (!def) continue;
        const wrap = document.createElement("div");
        wrap.className = "deck-view-card-wrap";
        wrap.dataset.cardId = choice.cardId;
        const cardEl = createPowerCard({
          type: def.type,
          name: def.name,
          description: def.description,
          example: def.example,
          mana: def.mana,
          ignition: def.ignition,
          cooldown: def.cooldown,
          cardWidth: "200px",
        });
        wrap.appendChild(cardEl);
        wrap.addEventListener("click", () => {
          if (!isGameplayInputOpen()) return;
          send("resolve_pending_effect", { targetCardId: choice.cardId });
          closeDeckSearchModal();
        });
        deckSearchGridEl.appendChild(wrap);
      }
    }

    deckSearchOverlayEl.classList.remove("hidden");
    deckSearchOverlayEl.setAttribute("aria-hidden", "false");
  }

  /** Closes the deck-search modal and resets its state. */
  function closeDeckSearchModal() {
    deckSearchOpenForCard = null;
    if (!deckSearchOverlayEl) return;
    deckSearchOverlayEl.classList.add("hidden");
    deckSearchOverlayEl.setAttribute("aria-hidden", "true");
    if (deckSearchGridEl) deckSearchGridEl.innerHTML = "";
  }

  /**
   * Syncs the deck-search modal visibility with the current snapshot.
   * Opens the modal when the local player has an archmage-arsenal pending effect;
   * closes it when the pending effect is gone.
   * @param {object} snapshot
   */
  function maybeUpdateDeckSearchModal(snapshot) {
    const self = playerEl?.value;
    if (!self || !snapshot) return;
    const pending = Array.isArray(snapshot.pendingEffects) ? snapshot.pendingEffects : [];
    const archmagePE = pending.find(
      (pe) => String(pe.owner || "") === self && String(pe.cardId || "") === "archmage-arsenal",
    );
    if (archmagePE) {
      if (!deckSearchOpenForCard) {
        openDeckSearchModal(archmagePE.deckSearchChoices || [], "archmage-arsenal");
      }
    } else {
      if (deckSearchOpenForCard) {
        closeDeckSearchModal();
      }
    }
  }

  async function openDeckViewModal() {
    const id = Number(lobbyDeckSelectEl.value, 10);
    if (!id || !readStoredToken()) return;
    try {
      const r = await fetch(`/api/decks/${id}`, {
        headers: { ...authFetchHeaders(), Accept: "application/json" },
      });
      if (!r.ok) return;
      const deck = await r.json();
      const cardIds = deck.cardIds || [];
      const counts = new Map();
      for (const cid of cardIds) {
        counts.set(cid, (counts.get(cid) || 0) + 1);
      }
      const catalog = getLocalizedCardCatalog(locale);
      const byId = new Map(catalog.map((c) => [c.id, c]));
      deckViewTitleEl.textContent = deck.name;
      deckViewGridEl.innerHTML = "";
      const sorted = [...counts.keys()].sort((a, b) => compareCardIdsByTypeThenName(a, b, byId));
      for (const cid of sorted) {
        const c = byId.get(cid);
        if (!c) continue;
        const wrap = document.createElement("div");
        wrap.className = "deck-view-card-wrap";
        const n = counts.get(cid);
        const badge = document.createElement("span");
        badge.className = `count-badge count-badge--${n}`;
        badge.textContent = `x${n}`;
        const cardEl = createPowerCard({
          type: c.type,
          name: c.name,
          description: c.description,
          example: c.example,
          mana: c.mana,
          ignition: c.ignition,
          cooldown: c.cooldown,
          cardWidth: "180px",
        });
        wrap.appendChild(cardEl);
        wrap.appendChild(badge);
        deckViewGridEl.appendChild(wrap);
      }
      deckViewModalEl.classList.remove("hidden");
      deckViewModalEl.setAttribute("aria-hidden", "false");
    } catch (_) {
      /* ignore */
    }
  }

  async function ensureHasDeckForMatch() {
    try {
      const r = await fetch("/api/decks", { headers: { ...authFetchHeaders(), Accept: "application/json" } });
      if (!r.ok) return true;
      const data = await r.json();
      const decks = data.decks || [];
      if (decks.length === 0) {
        alert(t("noSavedDeckAlert"));
        await refreshLobbyDecks();
        return false;
      }
      return true;
    } catch (_) {
      return true;
    }
  }

  function applyTranslations() {
    const s = (id, key) => {
      const el = document.getElementById(id);
      if (el) el.textContent = t(key);
    };

    s("titleLabel", "title");
    s("languageLabel", "language");
    s("roomNameLabel", "roomName");
    s("pieceTypeLabel", "pieceType");
    s("privateRoomLabel", "privateRoom");
    s("passwordLabel", "password");
    s("lobbyHint", "hint");
    s("roomListTitle", "openRooms");
    s("roomSearchLabel", "searchLabel");
    if (roomSearchEl) roomSearchEl.placeholder = t("searchPlaceholder");
    if (roomPasswordEl) roomPasswordEl.placeholder = t("passwordPlaceholder");
    if (roomListEmptyEl) roomListEmptyEl.textContent = t("noRooms");
    if (waitingBannerEl) waitingBannerEl.textContent = t("waiting");
    s("matchEndTitle", "matchFinished");
    if (matchEndRematchEl) matchEndRematchEl.textContent = t("playAgain");
    if (matchEndStayEl) matchEndStayEl.textContent = t("stayInRoom");
    if (matchEndToLobbyEl) matchEndToLobbyEl.textContent = t("backLobby");
    s("disconnectBtn", "leaveRoom");
    s("authTitle", "authCreateTitle");
    s("authUsernameLabel", "authUsername");
    s("authEmailLabel", "authEmail");
    s("authPasswordLabel", "authPassword");
    s("authConfirmPasswordLabel", "authConfirmPassword");
    s("authDividerLabel", "authAlreadyHave");
    s("authLoginEmailLabel", "authEmail");
    s("authLoginPasswordLabel", "authPassword");
    if (authRegisterBtnEl) authRegisterBtnEl.textContent = t("authRegister");
    if (authLoginBtnEl) authLoginBtnEl.textContent = t("authLogin");
    if (logoutBtnEl) logoutBtnEl.textContent = t("authLogout");
    s("reactionPendingTitle", "reactionPendingTitle");
    s("snapshotTitle", "snapshotTitle");
    s("eventsTitle", "eventsTitle");
    s("debugLogsTitle", "debugLogsTitle");
    s("handLabelSelf", "zoneHand");
    s("handLabelOpp", "zoneHand");
    s("deckLabelSelf", "zoneDeck");
    s("deckLabelOpp", "zoneDeck");
    s("labelIgnitionOpp", "zoneIgnition");
    s("labelIgnitionSelf", "zoneIgnition");
    s("labelCooldownOpp", "zoneCooldown");
    s("labelCooldownSelf", "zoneCooldown");
    s("labelBanishSelf", "zoneBanished");
    s("labelBanishOpp", "zoneBanished");
    s("labelGraveyardOpp", "zoneCapture");
    s("labelGraveyardSelf", "zoneCapture");
    if (pmEl.drawBtn) pmEl.drawBtn.textContent = t("drawFromDeck");
    if (lastSnapshot) renderMulliganBar(lastSnapshot);
    updateCoordsToggleLabel();
    updateEffectTurnsToggleLabel();
    updateReactionModeOptions();
    updateReactionModeLabel();
    if (reactionPassBtnEl) reactionPassBtnEl.textContent = t("reactionPassOk");
    if (exportClientTraceBtnEl) exportClientTraceBtnEl.textContent = t("exportClientTrace");
    if (pieceTypeEl) {
      const optRandom = pieceTypeEl.querySelector('option[value="random"]');
      const optWhite = pieceTypeEl.querySelector('option[value="white"]');
      const optBlack = pieceTypeEl.querySelector('option[value="black"]');
      if (optRandom) optRandom.textContent = t("pieceTypeRandom");
      if (optWhite) optWhite.textContent = t("pieceTypeWhite");
      if (optBlack) optBlack.textContent = t("pieceTypeBlack");
    }
    s("connectBtn", "create");
    syncPlayerRoleLabels();

    refreshLobbyUserLabel();
    renderRoomList(lobbyRooms);
    if (lastSnapshot) renderInRoomLabel(lastSnapshot);
    if (joinedRoom && lastSnapshot) updateOpponentDisconnectOverlay(lastSnapshot);
    updatePasswordToggleVisual();
    refreshPrivateJoinModalTexts();
    if (lobbyPrivatePasswordErrorEl && !lobbyPrivatePasswordErrorEl.classList.contains("hidden")) {
      lobbyPrivatePasswordErrorEl.textContent = t("privateNeedsPassword");
    }
    if (cardMarqueeLabelEl) cardMarqueeLabelEl.textContent = t("cardMarqueeTitle");
    s("lobbyDeckLabel", "lobbyDeckLabel");
    if (lobbyDeckViewBtnEl) lobbyDeckViewBtnEl.textContent = t("lobbyDeckView");
    if (lobbyDeckBuilderLinkEl) lobbyDeckBuilderLinkEl.textContent = t("lobbyDeckBuilder");
    if (deckViewCloseBtnEl) deckViewCloseBtnEl.textContent = t("deckViewClose");
    if (lobbyDeckHintEl)
      lobbyDeckHintEl.textContent = lobbyDeckRowEl?.classList?.contains("hidden") ? "" : t("lobbyDeckHint");
  }

  /**
   * Updates copy on the private-room join modal if it is open (e.g. after locale change).
   */
  function refreshPrivateJoinModalTexts() {
    if (privateJoinOverlayEl.classList.contains("hidden")) return;
    privateJoinTitleEl.textContent = t("privateJoinTitle");
    privateJoinBodyEl.textContent = t("privateJoinDescription", {
      name: privateJoinPendingRoomName || "Let's Play!",
    });
    privateJoinPasswordLabelEl.textContent = t("password");
    privateJoinPasswordInputEl.placeholder = t("passwordPlaceholder");
    privateJoinCancelEl.textContent = t("cancel");
    privateJoinSubmitEl.textContent = t("joinRoom");
    updatePrivateJoinToggleVisual();
    if (!privateJoinErrorEl.classList.contains("hidden")) {
      privateJoinErrorEl.textContent = t("privateJoinNeedsPassword");
    }
  }

  function updatePrivateJoinToggleVisual() {
    const showing = privateJoinPasswordInputEl.type === "text";
    privateJoinPasswordToggleEl.textContent = showing ? "🙈" : "👁";
    privateJoinPasswordToggleEl.setAttribute("aria-label", showing ? t("hide") : t("show"));
    privateJoinPasswordToggleEl.title = showing ? t("hide") : t("show");
  }

  function hidePrivateJoinError() {
    privateJoinErrorEl.textContent = "";
    privateJoinErrorEl.classList.add("hidden");
  }

  function showPrivateJoinError() {
    privateJoinErrorEl.textContent = t("privateJoinNeedsPassword");
    privateJoinErrorEl.classList.remove("hidden");
  }

  /**
   * Shows a small modal to collect the password for joining a private room.
   * Resolves with the trimmed password, or null if the user cancels.
   *
   * @param {string} roomName
   * @returns {Promise<string | null>}
   */
  function showPrivateJoinModal(roomName) {
    return new Promise((resolve) => {
      privateJoinPendingRoomName = roomName || "Let's Play!";
      const ac = new AbortController();
      const { signal } = ac;

      function close(result) {
        ac.abort();
        hidePrivateJoinError();
        privateJoinOverlayEl.classList.add("hidden");
        privateJoinOverlayEl.setAttribute("aria-hidden", "true");
        document.removeEventListener("keydown", onDocKey);
        resolve(result);
      }

      function onDocKey(ev) {
        if (ev.key === "Escape") close(null);
      }

      function refreshTexts() {
        privateJoinTitleEl.textContent = t("privateJoinTitle");
        privateJoinBodyEl.textContent = t("privateJoinDescription", { name: privateJoinPendingRoomName });
        privateJoinPasswordLabelEl.textContent = t("password");
        privateJoinPasswordInputEl.placeholder = t("passwordPlaceholder");
        privateJoinCancelEl.textContent = t("cancel");
        privateJoinSubmitEl.textContent = t("joinRoom");
        updatePrivateJoinToggleVisual();
      }

      function submit() {
        const pwd = String(privateJoinPasswordInputEl.value || "").trim();
        if (!pwd) {
          showPrivateJoinError();
          privateJoinPasswordInputEl.focus();
          return;
        }
        close(pwd);
      }

      refreshTexts();
      hidePrivateJoinError();
      privateJoinPasswordInputEl.value = "";
      privateJoinPasswordInputEl.type = "password";
      updatePrivateJoinToggleVisual();

      privateJoinOverlayEl.classList.remove("hidden");
      privateJoinOverlayEl.setAttribute("aria-hidden", "false");
      document.addEventListener("keydown", onDocKey);

      privateJoinOverlayEl.addEventListener(
        "click",
        (e) => {
          if (e.target === privateJoinOverlayEl) close(null);
        },
        { signal },
      );
      privateJoinCancelEl.addEventListener("click", () => close(null), { signal });
      privateJoinSubmitEl.addEventListener("click", () => submit(), { signal });
      privateJoinPasswordInputEl.addEventListener(
        "input",
        () => {
          hidePrivateJoinError();
        },
        { signal },
      );
      privateJoinPasswordInputEl.addEventListener(
        "keydown",
        (e) => {
          if (e.key === "Enter") submit();
        },
        { signal },
      );
      privateJoinPasswordToggleEl.addEventListener(
        "click",
        () => {
          privateJoinPasswordInputEl.type = privateJoinPasswordInputEl.type === "password" ? "text" : "password";
          updatePrivateJoinToggleVisual();
        },
        { signal },
      );

      queueMicrotask(() => privateJoinPasswordInputEl.focus());
    });
  }

  /** Fills reaction mode select option labels from the active locale. */
  function updateReactionModeOptions() {
    if (!reactionModeSelectEl || reactionModeSelectEl.options.length < 3) return;
    reactionModeSelectEl.options[0].textContent = t("reactionModeOff");
    reactionModeSelectEl.options[1].textContent = t("reactionModeOn");
    reactionModeSelectEl.options[2].textContent = t("reactionModeAuto");
  }

  function updateReactionModeLabel() {
    if (!reactionModeSelectEl || !reactionModeLabelEl) return;
    const mode = reactionModeSelectEl.value;
    let modeKey = "reactionModeOn";
    if (mode === "off") modeKey = "reactionModeOff";
    else if (mode === "auto") modeKey = "reactionModeAuto";
    reactionModeLabelEl.textContent = `${t("reactions")}: ${t(modeKey)}`;
  }

  /**
   * Syncs the header control from the server snapshot (per-player reactionMode).
   * @param {object} snapshot
   */
  function syncReactionModeFromSnapshot(snapshot) {
    if (!reactionModeSelectEl || !snapshot?.players) return;
    const selfId = playerEl.value;
    const row = snapshot.players.find((p) => p.playerId === selfId);
    const raw = (row?.reactionMode || "on").toLowerCase();
    const m = raw === "off" || raw === "auto" || raw === "on" ? raw : "on";
    if (reactionModeSelectEl.value !== m) {
      reactionModeSelectEl.value = m;
      updateReactionModeLabel();
    }
  }

  function updateCoordsToggleLabel() {
    if (!coordsInSquaresEl || !coordsInSquaresTextEl) return;
    coordsInSquaresTextEl.textContent = `${t("coordsLabel")}: ${coordsInSquaresEl.checked ? t("toggleOn") : t("toggleOff")}`;
  }

  function updateEffectTurnsToggleLabel() {
    if (!effectTurnsAlwaysVisibleEl || !effectTurnsToggleTextEl) return;
    const mode = effectTurnsAlwaysVisibleEl.checked ? t("effectTurnsAlways") : t("effectTurnsHover");
    effectTurnsToggleTextEl.textContent = `${t("effectTurnsLabel")}: ${mode}`;
  }

  /**
   * Match HUD seat line: server display name only (no role prefix).
   * @param {string} [name] snapshot display name for that seat
   * @returns {string}
   */
  function playerDisplayNameLabel(name) {
    const n = name && String(name).trim();
    return n || "-";
  }

  /**
   * @param {object} [snapshot] state_snapshot payload; falls back to lastSnapshot when omitted
   */
  function syncPlayerRoleLabels(snapshot) {
    const snap = snapshot !== undefined ? snapshot : lastSnapshot;
    if (!playerEl) return;
    const top = document.getElementById("playerBLabel");
    const bottom = document.getElementById("playerALabel");
    if (!top || !bottom) return;
    const nameA = snap?.playerAName ?? "";
    const nameB = snap?.playerBName ?? "";
    top.textContent = playerDisplayNameLabel(nameB);
    bottom.textContent = playerDisplayNameLabel(nameA);
  }

  /**
   * Shows or hides the card marquee footer (lobby-only).
   * @param {boolean} visible
   */
  function setLobbyFooterVisible(visible) {
    if (!mainFooterEl) return;
    mainFooterEl.classList.toggle("hidden", !visible);
    document.body.classList.toggle("has-card-footer", visible);
  }

  function setLocale(nextLocale) {
    locale = i18n[nextLocale] ? nextLocale : "en-US";
    if (localeSelectEl) localeSelectEl.value = locale;
    try {
      localStorage.setItem("powerChessLocale", locale);
    } catch (_) {
      /* ignore storage failures */
    }
    applyTranslations();
    document.dispatchEvent(new CustomEvent("powerchess:locale", { detail: { locale } }));
  }

  function hideLobbyPrivatePasswordError() {
    if (!lobbyPrivatePasswordErrorEl) return;
    lobbyPrivatePasswordErrorEl.textContent = "";
    lobbyPrivatePasswordErrorEl.classList.add("hidden");
  }

  function showLobbyPrivatePasswordError() {
    if (!lobbyPrivatePasswordErrorEl) return;
    lobbyPrivatePasswordErrorEl.textContent = t("privateNeedsPassword");
    lobbyPrivatePasswordErrorEl.classList.remove("hidden");
  }

  function updatePrivatePasswordVisibility() {
    if (!roomPasswordFieldEl || !privateRoomEl) return;
    roomPasswordFieldEl.classList.toggle("hidden", !privateRoomEl.checked);
    if (!privateRoomEl.checked) {
      hideLobbyPrivatePasswordError();
    }
  }

  function updatePasswordToggleVisual() {
    if (!roomPasswordEl || !roomPasswordToggleEl) return;
    const showing = roomPasswordEl.type === "text";
    roomPasswordToggleEl.textContent = showing ? "🙈" : "👁";
    roomPasswordToggleEl.setAttribute("aria-label", showing ? t("hide") : t("show"));
    roomPasswordToggleEl.title = showing ? t("hide") : t("show");
  }

  function chooseRandomPlayer() {
    return Math.random() < 0.5 ? "A" : "B";
  }

  function desiredPlayerForPieceType(pieceType) {
    if (pieceType === "white") return "A";
    if (pieceType === "black") return "B";
    return chooseRandomPlayer();
  }

  function oppositePieceType(pieceType) {
    if (pieceType === "white") return "black";
    if (pieceType === "black") return "white";
    return "random";
  }

  function isJoinOccupiedSideError(msg) {
    if (!msg || msg.type !== "error") return false;
    const errCode = String(msg.payload?.code || "");
    const errMessage = String(msg.payload?.message || "").toLowerCase();
    if (errCode !== "action_failed") return false;
    return errMessage.includes("already occupied");
  }

  function socketBaseURL() {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${location.host}/ws`;
  }

  /** Same JWT as REST; required by the server when DATABASE_URL auth is enabled. */
  function socketURLWithAuth() {
    const base = socketBaseURL();
    if (!authBackendAvailable) return base;
    const tok = readStoredToken();
    if (!tok) return base;
    const u = new URL(base);
    u.searchParams.set("token", tok);
    return u.toString();
  }

  /**
   * Opens the WebSocket and sends join_match.
   * @param {string} roomId
   * @param {string} [pieceTypeOverride]
   * @param {string} [roomNameOverride]
   * @param {boolean} [privateOverride]
   * @param {string} [passwordOverride]
   */
  async function connectToRoom(roomId, pieceTypeOverride, roomNameOverride, privateOverride, passwordOverride) {
    if (readStoredToken() && authBackendAvailable) {
      if (!(await ensureHasDeckForMatch())) return;
    }
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
    joinedRoom = false;
    gameStarted = false;
    prevMatchEnded = false;
    hideMatchEndOverlay();
    hideLobbyPrivatePasswordError();
    // API /api/rooms may return roomId as a JSON number; always send a string for join_match.
    const roomIdStr = String(roomId ?? "").trim();
    const pieceType = pieceTypeOverride || (pieceTypeEl ? pieceTypeEl.value : "random") || "random";
    const roomName =
      (roomNameOverride || (roomNameEl ? roomNameEl.value : "Let's Play!") || "Let's Play!").trim() || "Let's Play!";
    const creatingNewRoom = !roomIdStr;
    const isPrivate =
      typeof privateOverride === "boolean"
        ? privateOverride
        : creatingNewRoom
          ? privateRoomEl
            ? privateRoomEl.checked
            : false
          : false;
    let password = "";
    if (typeof passwordOverride === "string") {
      password = passwordOverride;
    } else if (creatingNewRoom && roomPasswordEl) {
      password = roomPasswordEl.value;
    }
    if (isPrivate && !String(password || "").trim()) {
      showLobbyPrivatePasswordError();
      if (roomPasswordEl) roomPasswordEl.focus();
      return;
    }
    playerEl.value = desiredPlayerForPieceType(pieceType);
    pendingJoinAttempt = {
      roomId: roomIdStr,
      roomName,
      isPrivate,
      password,
      pieceType,
      attemptedFallback: false,
    };
    syncPlayerRoleLabels();
    ws = new WebSocket(socketURLWithAuth());
    ws.onopen = () => {
      logEvent({ event: "socket_open" });
      matchTestFixtureSent = false;
      matchTestPrevBothConnected = false;
      matchTestAwaitingMulliganConfirm = false;
      send("join_match", {
        roomId: roomIdStr,
        roomName,
        pieceType,
        playerId: playerEl.value,
        isPrivate,
        password,
      });
    };
    ws.onclose = () => {
      if (clientTraceMirrorTimerId) {
        clearInterval(clientTraceMirrorTimerId);
        clientTraceMirrorTimerId = null;
      }
      logEvent({ event: "socket_close" });
      stopRoomListPolling();
      resetToLobbyUi();
    };
    ws.onerror = () => logEvent({ event: "socket_error" });
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data);
      logEvent(msg);
      // Server→client: one card finished its activation step (reaction pile LIFO or ignition counter0).
      if (msg.type === "activate_card" && msg.payload && typeof msg.payload === "object") {
        bufferServerActivationFx(msg.payload);
        return;
      }
      if (msg.type === "state_snapshot") {
        pendingJoinAttempt = null;
        const nextSnap = msg.payload;
        lastSnapshot = nextSnap;
        // Clear disruption banish flow if reaction window closed or changed.
        if (disruptionBanishFlow !== null) {
          const rw = nextSnap?.reactionWindow;
          if (!rw?.open || rw?.trigger !== "ignite_reaction") {
            disruptionBanishFlow = null;
          }
        }
        if (retaliateTargetFlow !== null) {
          const rw = nextSnap?.reactionWindow;
          if (!rw?.open || !hasValidRetaliateTarget(nextSnap, playerEl.value)) {
            retaliateTargetFlow = null;
          }
        }
        if (copiedZipLineSource && !viewerPendingBoardTargetEffect(nextSnap) && !viewerZipLinePendingSource(nextSnap)) {
          copiedZipLineSource = null;
        }
        maybeAdvanceIgniteTargetFlow(nextSnap);
        maybeClearIgniteTargetFlow(nextSnap);
        syncIgnitionBlueHoldFromSnapshot(nextSnap);
        const viewer = nextSnap.viewerPlayerId;
        if ((viewer === "A" || viewer === "B") && playerEl && playerEl.value !== viewer) {
          playerEl.value = viewer;
          syncBoardPerspectiveClass();
          syncPlayerRoleLabels(nextSnap);
        }
        selectedFrom = null;
        highlightedMoves = [];

        if (!joinedRoom) {
          joinedRoom = true;
          if (lobbyScreenEl) {
            setLobbyFooterVisible(false);
            lobbyScreenEl.classList.add("hidden");
          }
          if (gameShellEl) gameShellEl.classList.remove("hidden");
          if (playerEl) playerEl.disabled = true;
          if (roomNameEl) roomNameEl.disabled = true;
          if (roomSearchEl) roomSearchEl.disabled = true;
          if (privateRoomEl) privateRoomEl.disabled = true;
          if (roomPasswordEl) roomPasswordEl.disabled = true;
          if (pieceTypeEl) pieceTypeEl.disabled = true;
          stopRoomListPolling();
          syncBoardPerspectiveClass();
        }

        updateLobbyChromeFromSnapshot(nextSnap);
        updateOpponentDisconnectOverlay(nextSnap);
        maybeShowMatchEndModal(nextSnap);
        updateReactionPassButton(nextSnap);

        if (nextSnap.adminDebugMatch) {
          if (!clientTraceMirrorTimerId) {
            clientTraceMirrorTimerId = setInterval(flushClientTraceToServer, 8000);
          }
        } else if (clientTraceMirrorTimerId) {
          clearInterval(clientTraceMirrorTimerId);
          clientTraceMirrorTimerId = null;
        }

        if (snapshotEl) snapshotEl.textContent = JSON.stringify(nextSnap, null, 2);
        // Defer board render only for reaction-stack resolution (capture/ignite chains).
        // Do not defer for pending activate_card alone: the move must show first, then
        // runTurnStartResourceSequence (ignition tick → cooldown → other CDs), then activate_card FX.
        // prevReceivedSnapshot tracks the last-received snapshot synchronously so rapid bursts
        // of snapshots still see the correct transition.
        const willReactionResolve = shouldRunReactionStackResolve(prevReceivedSnapshot, nextSnap);
        const willDeferBoard = willReactionResolve;
        prevReceivedSnapshot = nextSnap;
        if (!willDeferBoard) {
          renderBoard(nextSnap.board);
        }
        renderStatus(nextSnap);
        // Player HUD is updated in enqueueSnapshotApply's finally (see enqueueSnapshotApply) so a
        // burst of state_snapshot messages cannot paint post-flush mana while an earlier snapshot's
        // banner/activation-FX chain is still awaiting (e.g. Mana Burn after peer client_fx_release).

        enqueueSnapshotApply(async () => {
          const prevSnap = pmPrevSnapshot;
          const turnChanged =
            prevSnap &&
            nextSnap.gameStarted &&
            prevSnap.turnPlayer &&
            nextSnap.turnPlayer &&
            prevSnap.turnPlayer !== nextSnap.turnPlayer;

          const doReactionResolve = shouldRunReactionStackResolve(prevSnap, nextSnap);
          if (!doReactionResolve) {
            /** @type {object | null} */
            let ignitionCooldownDeferral = null;
            if (turnChanged) {
              send("client_fx_hold", {});
              turnResourceAnimBlocking = true;
              try {
                ignitionCooldownDeferral = await runTurnStartResourceSequence(prevSnap, nextSnap, {
                  deferCooldownUntilAfterActivate: true,
                });
              } finally {
                turnResourceAnimBlocking = false;
                send("client_fx_release", {});
              }
            }
            const deferAttached = flushPendingActivationFx(null, ignitionCooldownDeferral);
            await waitActivationFxWorkerIdle();
            if (ignitionCooldownDeferral?.deferCooldownAfterActivation && !deferAttached) {
              await applyIgnitionResolveThenCooldownOdometers(ignitionCooldownDeferral);
              const pid = ignitionCooldownDeferral.turnStarter;
              const isSelf = pid === playerEl.value;
              const clearedIgnitionCardEl = isSelf ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
              const clearedIgnitionCounterEl = isSelf ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;
              if (clearedIgnitionCardEl) {
                clearedIgnitionCardEl.innerHTML = "";
                delete clearedIgnitionCardEl.dataset.cardId;
                clearedIgnitionCardEl.classList.remove("pm-ignition-staged");
                clearedIgnitionCardEl.querySelectorAll(".pm-ignition-staged-overlay").forEach((el) => el.remove());
              }
              if (clearedIgnitionCounterEl) clearedIgnitionCounterEl.classList.add("hidden");
              renderIgnitionZone(nextSnap);
            }
            if (willDeferBoard) {
              renderBoard(nextSnap.board);
            }
          }
          if (doReactionResolve) {
            const bannerCount = reactionStackBannerCardCount(prevSnap);
            const stepsPreview = reactionStackResolveOrderFromPrev(prevSnap);
            const reactionBlockMs =
              (bannerCount > 1 ? 1500 : 0) + Math.max(stepsPreview.length, bannerCount, 1) * 1520 + 300;
            try {
              await runReactionStackResolveSequence(prevSnap, nextSnap);
            } catch (_) {
              flushPendingActivationFx(prevSnap);
            }
            // Ensure board mutation (captured piece removed) only appears after all
            // server-driven activation FX for this resolve step are fully finished.
            await waitActivationFxWorkerIdle();
            // Board was deferred: apply now that all reaction animations finished.
            renderBoard(nextSnap.board);
            let effectBlockMs = reactionBlockMs;
            if (hasEffectAnimationDelta(prevSnap, nextSnap)) {
              effectBlockMs = Math.max(effectBlockMs, 900);
            }
            blockGameplayInputForEffects(effectBlockMs);
          }

          if (turnChanged && doReactionResolve) {
            send("client_fx_hold", {});
            turnResourceAnimBlocking = true;
            try {
              await runTurnStartResourceSequence(prevSnap, nextSnap);
            } finally {
              turnResourceAnimBlocking = false;
              send("client_fx_release", {});
            }
          }

          const skipDupPlaymatAnim =
            prevSnap && snapshotPlaymatAnimationDigest(prevSnap) === snapshotPlaymatAnimationDigest(nextSnap);
          if (!skipDupPlaymatAnim) {
            runSnapshotAnimations(prevSnap, nextSnap, doReactionResolve);
          }
          if (!doReactionResolve && !skipDupPlaymatAnim && hasEffectAnimationDelta(prevSnap, nextSnap)) {
            blockGameplayInputForEffects(900);
          }
          maybeUpdateDeckSearchModal(nextSnap);
          renderPlaymat(nextSnap);
          pmPrevSnapshot = nextSnap;
          syncPlayerRoleLabels(nextSnap);
          syncReactionModeFromSnapshot(nextSnap);
          maybeApplyMatchTestFixture(nextSnap);
          requestAnimationFrame(() => {
            requestAnimationFrame(() => renderCaptureThreatOverlay());
          });
        });
        return;
      }
      if (isJoinOccupiedSideError(msg) && !joinedRoom && pendingJoinAttempt && !pendingJoinAttempt.attemptedFallback) {
        pendingJoinAttempt.attemptedFallback = true;
        pendingJoinAttempt.pieceType = oppositePieceType(pendingJoinAttempt.pieceType);
        playerEl.value = desiredPlayerForPieceType(pendingJoinAttempt.pieceType);
        syncPlayerRoleLabels();
        send("join_match", {
          roomId: pendingJoinAttempt.roomId,
          roomName: pendingJoinAttempt.roomName,
          pieceType: pendingJoinAttempt.pieceType,
          playerId: playerEl.value,
          isPrivate: pendingJoinAttempt.isPrivate,
          password: pendingJoinAttempt.password,
        });
      }
    };
  }

  const pieceTypeToPng = {
    K: "King",
    Q: "Queen",
    R: "Rook",
    B: "Bishop",
    N: "Knight",
    P: "Pawn",
  };
  const promotionChoices = [
    { value: "queen", type: "Q", labelKey: "promotionQueen" },
    { value: "rook", type: "R", labelKey: "promotionRook" },
    { value: "bishop", type: "B", labelKey: "promotionBishop" },
    { value: "knight", type: "N", labelKey: "promotionKnight" },
  ];

  /**
   * pieceImageURL maps engine codes (wK, bQ) to PNG paths under /public/pieces/.
   * @param {string} code
   * @returns {string}
   */
  function pieceImageURL(code) {
    if (!code || code.length < 2) return "";
    const color = code[0];
    const type = code[1];
    if (color !== "w" && color !== "b") return "";
    const name = pieceTypeToPng[type];
    if (!name) return "";
    return `/public/pieces/${color}${name}.png`;
  }

  function posKey(row, col) {
    return `${row},${col}`;
  }

  function parseCode(code) {
    if (!code || code.length < 2) return null;
    return { color: code[0], type: code[1] };
  }

  /** Maps transport piece codes ("wK", "bP") to seat "A" (white) or "B" (black). */
  function seatForPieceCode(code) {
    const p = parseCode(code);
    if (!p) return "";
    return p.color === "w" ? "A" : "B";
  }

  function inBounds(row, col) {
    return row >= 0 && row < 8 && col >= 0 && col < 8;
  }

  function isBoardFlipped() {
    return playerEl.value === "B";
  }

  function displayToLogical(row, col) {
    if (!isBoardFlipped()) return { row, col };
    return { row: 7 - row, col: 7 - col };
  }

  function logicalToAlgebraic(row, col) {
    const file = String.fromCharCode(97 + col);
    const rank = 8 - row;
    return `${file}${rank}`;
  }

  /** Zip Line pending effect for the local seat (server sends sourceRow/sourceCol on state_snapshot). */
  function viewerZipLinePendingSource(snapshot) {
    const self = playerEl.value;
    if (!snapshot || !Array.isArray(snapshot.pendingEffects)) return null;
    for (const pe of snapshot.pendingEffects) {
      if (String(pe.owner || "") !== self || String(pe.cardId || "") !== "zip-line") continue;
      if (copiedZipLineSource) return copiedZipLineSource;
      if (pe.sourceRow == null || pe.sourceCol == null) continue;
      return { row: Number(pe.sourceRow), col: Number(pe.sourceCol) };
    }
    return null;
  }

  /** Returns a pending copied effect that needs a board piece selection from the local player. */
  function viewerPendingBoardTargetEffect(snapshot) {
    const self = playerEl.value;
    if (!snapshot || !Array.isArray(snapshot.pendingEffects)) return null;
    for (const pe of snapshot.pendingEffects) {
      if (String(pe.owner || "") !== self) continue;
      const cardId = String(pe.cardId || "");
      const def = getCardDef(cardId);
      if ((Number(def?.targets) || 0) <= 0) continue;
      if (cardId === "zip-line" && copiedZipLineSource) continue;
      if (cardId === "zip-line" && pe.sourceRow != null && pe.sourceCol != null) continue;
      return { cardId };
    }
    return null;
  }

  /** Empty squares on the same rank as `src` (excluding `src`), using logical board indices. */
  function zipLineDestKeySet(board, src) {
    const set = new Set();
    if (!src || !board || !Array.isArray(board)) return set;
    const r = src.row;
    for (let c = 0; c < 8; c++) {
      if (c === src.col) continue;
      const cell = board[r]?.[c];
      if (cell && String(cell).trim()) continue;
      set.add(posKey(r, c));
    }
    return set;
  }

  /**
   * Lightweight client preflight: own non-king with some empty square on the same rank.
   * Server still validates check / legality.
   */
  function zipLineHasRoughPlayableTarget(board, seatId) {
    if (!board || !Array.isArray(board)) return false;
    const oc = seatId === "A" ? "w" : "b";
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const code = board[r]?.[c] || "";
        if (!code || code[0] !== oc || code[1] === "K") continue;
        for (let c2 = 0; c2 < 8; c2++) {
          if (c2 === c) continue;
          if (!board[r][c2]) return true;
        }
      }
    }
    return false;
  }

  /** Locked ignition target squares from snapshot (top-level + reaction window mirror). */
  function boardIgnitionTargetPieces(snapshot) {
    const out = [];
    const igt = snapshot?.ignitionTargeting;
    if (Array.isArray(igt?.target_pieces)) out.push(...igt.target_pieces);
    const rw = snapshot?.reactionWindow;
    if (Array.isArray(rw?.target_pieces)) {
      for (const tp of rw.target_pieces) {
        if (!out.some((x) => Number(x.row) === Number(tp.row) && Number(x.col) === Number(tp.col))) {
          out.push(tp);
        }
      }
    }
    return out;
  }

  function syncIgnitionBlueHoldFromSnapshot(snapshot) {
    const pieces = boardIgnitionTargetPieces(snapshot);
    const igt = snapshot?.ignitionTargeting;
    const rw = snapshot?.reactionWindow;
    const owner = igt?.owner || rw?.targetingOwner;
    const cardId = igt?.cardId || rw?.targetingCardId;
    if (pieces.length > 0 && owner && cardId) {
      ignitionBlueHold = {
        owner: String(owner),
        cardId: String(cardId),
        pieces: pieces.map((tp) => ({ row: Number(tp.row), col: Number(tp.col) })),
      };
    }
  }

  function maybeAdvanceIgniteTargetFlow(snapshot) {
    if (!igniteTargetFlow || igniteTargetFlow.stage !== "placed") return;
    const igt = snapshot?.ignitionTargeting;
    const self = playerEl.value;
    if (!igt || igt.owner !== self || String(igt.cardId || "") !== igniteTargetFlow.cardId) return;
    if (igt.awaitingTargetChoice) igniteTargetFlow.stage = "picking";
  }

  function maybeClearIgniteTargetFlow(snapshot) {
    if (!igniteTargetFlow) return;
    const igt = snapshot?.ignitionTargeting;
    const self = playerEl.value;
    if (
      igt?.owner === self &&
      Array.isArray(igt.target_pieces) &&
      igt.target_pieces.length > 0 &&
      !igt.awaitingTargetChoice
    ) {
      igniteTargetFlow = null;
      return;
    }
    const selfHud = snapshot?.players?.find((p) => p.playerId === self);
    if (selfHud && !selfHud.ignitionOn && igniteTargetFlow) {
      igniteTargetFlow = null;
    }
  }

  function ignitionDottedPiecesForUI(snapshot) {
    const fromSnap = boardIgnitionTargetPieces(snapshot);
    if (fromSnap.length > 0) return fromSnap;
    if (!ignitionBlueHold) return [];
    const matchPl = (pl) =>
      String(pl.playerId || "") === ignitionBlueHold.owner && String(pl.cardId || "") === ignitionBlueHold.cardId;
    const pending =
      pendingActivateCardPayloads.some(matchPl) ||
      activationFxQueue.some(matchPl) ||
      activationFxWorkerPromise !== null;
    if (pending) return ignitionBlueHold.pieces;
    return [];
  }

  /**
   * Returns all active power-type per-piece effects that apply to the square at (r, c).
   * Replaces the former single-result pieceActivePowerAura.
   */
  function pieceActivePowerAuras(snapshot, r, c) {
    const fx = snapshot?.activePieceEffects;
    if (!Array.isArray(fx)) return [];
    const result = [];
    for (const e of fx) {
      if (Number(e.turnsRemaining || 0) <= 0) continue;
      if (Number(e.row) !== r || Number(e.col) !== c) continue;
      const t = String(getCardDef(e.cardId)?.type || "").toLowerCase();
      if (t === "power") result.push(e);
    }
    return result;
  }

  function fileLetterFromDisplayEdge(displayRow, displayCol) {
    const { col } = displayToLogical(displayRow, displayCol);
    return String.fromCharCode(97 + col);
  }

  function rankDigitFromDisplayEdge(displayRow, displayCol) {
    const { row } = displayToLogical(displayRow, displayCol);
    return String(8 - row);
  }

  function pieceAt(board, row, col) {
    if (!inBounds(row, col)) return "";
    return board?.[row]?.[col] || "";
  }

  /** Returns the Chebyshev distance between two board positions. */
  function chebyshevDist(a, b) {
    return Math.max(Math.abs(a.row - b.row), Math.abs(a.col - b.col));
  }

  /**
   * Returns true if there is any piece between `from` and `to` along the direction (dr, dc).
   * Assumes from and to are distinct and that `to` is reachable from `from` by stepping (dr, dc).
   */
  function rayBlocked(board, from, dr, dc, to) {
    let r = from.row + dr;
    let c = from.col + dc;
    while (r !== to.row || c !== to.col) {
      if (!inBounds(r, c)) return true;
      if (pieceAt(board, r, c)) return true;
      r += dr;
      c += dc;
    }
    return false;
  }

  /**
   * Returns true if the piece of given type/color at `from` can attack `to` on the board.
   * Does not validate legality beyond attack pattern.
   */
  function pieceAttacks(board, type, color, from, to) {
    const dr = to.row - from.row;
    const dc = to.col - from.col;
    if (dr === 0 && dc === 0) return false;
    switch (type) {
      case "P": {
        const dir = color === "b" ? 1 : -1;
        return dr === dir && (dc === 1 || dc === -1);
      }
      case "N":
        return (Math.abs(dr) === 2 && Math.abs(dc) === 1) || (Math.abs(dr) === 1 && Math.abs(dc) === 2);
      case "K":
        return Math.abs(dr) <= 1 && Math.abs(dc) <= 1;
      case "B":
        if (Math.abs(dr) !== Math.abs(dc)) return false;
        return !rayBlocked(board, from, Math.sign(dr), Math.sign(dc), to);
      case "R":
        if (dr !== 0 && dc !== 0) return false;
        return !rayBlocked(board, from, Math.sign(dr), Math.sign(dc), to);
      case "Q":
        if (dr !== 0 && dc !== 0 && Math.abs(dr) !== Math.abs(dc)) return false;
        return !rayBlocked(board, from, Math.sign(dr), Math.sign(dc), to);
      default:
        return false;
    }
  }

  /**
   * Returns true if `color`'s king is in check on the given board.
   * Board is the flat 8×8 array from the snapshot.
   */
  function isKingInCheck(board, color) {
    let kingRow = -1,
      kingCol = -1;
    outer: for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const p = parseCode(pieceAt(board, r, c));
        if (p && p.color === color && p.type === "K") {
          kingRow = r;
          kingCol = c;
          break outer;
        }
      }
    }
    if (kingRow < 0) return false;
    const oppColor = color === "w" ? "b" : "w";
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const p = parseCode(pieceAt(board, r, c));
        if (!p || p.color !== oppColor) continue;
        if (pieceAttacks(board, p.type, p.color, { row: r, col: c }, { row: kingRow, col: kingCol })) {
          return true;
        }
      }
    }
    return false;
  }

  /**
   * Returns true if swapping pieces at pos1 and pos2 would put `ownColor`'s king in check.
   * Performs a lightweight board clone for simulation.
   */
  function isKingInCheckAfterSwap(board, pos1, pos2, ownColor) {
    const b = board.map((row) => [...row]);
    const tmp = b[pos1.row][pos1.col];
    b[pos1.row][pos1.col] = b[pos2.row][pos2.col];
    b[pos2.row][pos2.col] = tmp;
    return isKingInCheck(b, ownColor);
  }

  /**
   * Returns opponent piece positions within Chebyshev 2 of `ownPos` that are valid
   * second targets for Piece Swap (non-king, and swap does not expose own king).
   */
  function pieceSwapValidTargets(board, ownPos, ownColor) {
    const oppColor = ownColor === "w" ? "b" : "w";
    const out = [];
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const p = parseCode(pieceAt(board, r, c));
        if (!p || p.color !== oppColor || p.type === "K") continue;
        if (chebyshevDist(ownPos, { row: r, col: c }) > 2) continue;
        if (!isKingInCheckAfterSwap(board, ownPos, { row: r, col: c }, ownColor)) {
          out.push({ row: r, col: c });
        }
      }
    }
    return out;
  }

  /**
   * Returns a Set of posKey strings for own non-king pieces that have at least one valid
   * Piece Swap target (used to highlight eligible first-pick squares).
   */
  function pieceSwapEligibleOwnSet(board, ownColor) {
    const result = new Set();
    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const p = parseCode(pieceAt(board, r, c));
        if (!p || p.color !== ownColor || p.type === "K") continue;
        if (pieceSwapValidTargets(board, { row: r, col: c }, ownColor).length > 0) {
          result.add(posKey(r, c));
        }
      }
    }
    return result;
  }

  function pushIfValidMove(out, board, color, row, col) {
    if (!inBounds(row, col)) return false;
    const dst = parseCode(pieceAt(board, row, col));
    if (!dst) {
      out.push({ row, col });
      return true;
    }
    if (dst.color !== color) out.push({ row, col });
    return false;
  }

  function slidingMoves(out, board, color, from, deltas) {
    for (const [dr, dc] of deltas) {
      let r = from.row + dr;
      let c = from.col + dc;
      while (inBounds(r, c)) {
        const keep = pushIfValidMove(out, board, color, r, c);
        if (!keep) break;
        r += dr;
        c += dc;
      }
    }
  }

  function isSquareAttackedBy(board, row, col, byColor) {
    const knightJumps = [
      [-2, -1],
      [-2, 1],
      [-1, -2],
      [-1, 2],
      [1, -2],
      [1, 2],
      [2, -1],
      [2, 1],
    ];
    const kingSteps = [
      [-1, -1],
      [-1, 0],
      [-1, 1],
      [0, -1],
      [0, 1],
      [1, -1],
      [1, 0],
      [1, 1],
    ];

    for (let r = 0; r < 8; r++) {
      for (let c = 0; c < 8; c++) {
        const src = parseCode(pieceAt(board, r, c));
        if (!src || src.color !== byColor) continue;

        if (src.type === "P") {
          const dir = byColor === "w" ? -1 : 1;
          if (r + dir === row && (c - 1 === col || c + 1 === col)) return true;
          continue;
        }

        if (src.type === "N") {
          for (const [dr, dc] of knightJumps) {
            if (r + dr === row && c + dc === col) return true;
          }
          continue;
        }

        if (src.type === "K") {
          for (const [dr, dc] of kingSteps) {
            if (r + dr === row && c + dc === col) return true;
          }
          continue;
        }

        const rayDirs = [];
        if (src.type === "B" || src.type === "Q") rayDirs.push([-1, -1], [-1, 1], [1, -1], [1, 1]);
        if (src.type === "R" || src.type === "Q") rayDirs.push([-1, 0], [1, 0], [0, -1], [0, 1]);
        if (!rayDirs.length) continue;

        for (const [dr, dc] of rayDirs) {
          let rr = r + dr;
          let cc = c + dc;
          while (inBounds(rr, cc)) {
            if (rr === row && cc === col) return true;
            if (parseCode(pieceAt(board, rr, cc))) break;
            rr += dr;
            cc += dc;
          }
        }
      }
    }
    return false;
  }

  /** Merges knight-pattern destinations when `activePieceEffects` grants knight-touch on this square. */
  function mergeKnightTouchGrant(out, board, from, color, snapshot) {
    if (!snapshot) return out;
    const localPid = playerEl.value;
    const fx = snapshot.activePieceEffects;
    if (!Array.isArray(fx)) return out;
    const src = parseCode(pieceAt(board, from.row, from.col));
    if (!src || src.type === "N" || src.type === "K") return out;
    const localColor = localPid === "A" ? "w" : "b";
    if (color !== localColor) return out;
    const has = fx.some(
      (e) =>
        e.owner === localPid &&
        String(e.cardId || "") === "knight-touch" &&
        Number(e.turnsRemaining || 0) > 0 &&
        Number(e.row) === from.row &&
        Number(e.col) === from.col,
    );
    if (!has) return out;
    const jumps = [
      [-2, -1],
      [-2, 1],
      [-1, -2],
      [-1, 2],
      [1, -2],
      [1, 2],
      [2, -1],
      [2, 1],
    ];
    for (const [dr, dc] of jumps) {
      pushIfValidMove(out, board, color, from.row + dr, from.col + dc);
    }
    return out;
  }

  /** Merges bishop-pattern destinations when `activePieceEffects` grants bishop-touch on this square. */
  function mergeBishopTouchGrant(out, board, from, color, snapshot) {
    if (!snapshot) return out;
    const localPid = playerEl.value;
    const fx = snapshot.activePieceEffects;
    if (!Array.isArray(fx)) return out;
    const src = parseCode(pieceAt(board, from.row, from.col));
    if (!src || src.type === "B" || src.type === "K") return out;
    const localColor = localPid === "A" ? "w" : "b";
    if (color !== localColor) return out;
    const has = fx.some(
      (e) =>
        e.owner === localPid &&
        String(e.cardId || "") === "bishop-touch" &&
        Number(e.turnsRemaining || 0) > 0 &&
        Number(e.row) === from.row &&
        Number(e.col) === from.col,
    );
    if (!has) return out;
    const maxSteps = src.type === "P" ? 1 : 8;
    const diags = [
      [-1, -1],
      [-1, 1],
      [1, -1],
      [1, 1],
    ];
    for (const [dr, dc] of diags) {
      let r = from.row + dr;
      let c = from.col + dc;
      let traveled = 0;
      while (inBounds(r, c) && traveled < maxSteps) {
        const dst = parseCode(pieceAt(board, r, c));
        if (!dst) {
          out.push({ row: r, col: c });
          traveled++;
          r += dr;
          c += dc;
          continue;
        }
        if (dst.color !== color) {
          out.push({ row: r, col: c });
        }
        break;
      }
    }
    return out;
  }

  /** Merges rook-pattern destinations when `activePieceEffects` grants rook-touch on this square. */
  function mergeRookTouchGrant(out, board, from, color, snapshot) {
    if (!snapshot) return out;
    const localPid = playerEl.value;
    const fx = snapshot.activePieceEffects;
    if (!Array.isArray(fx)) return out;
    const src = parseCode(pieceAt(board, from.row, from.col));
    if (!src || src.type === "R" || src.type === "K") return out;
    const localColor = localPid === "A" ? "w" : "b";
    if (color !== localColor) return out;
    const has = fx.some(
      (e) =>
        e.owner === localPid &&
        String(e.cardId || "") === "rook-touch" &&
        Number(e.turnsRemaining || 0) > 0 &&
        Number(e.row) === from.row &&
        Number(e.col) === from.col,
    );
    if (!has) return out;
    const maxSteps = src.type === "P" ? 1 : 8;
    const orth = [
      [-1, 0],
      [1, 0],
      [0, -1],
      [0, 1],
    ];
    for (const [dr, dc] of orth) {
      let r = from.row + dr;
      let c = from.col + dc;
      let traveled = 0;
      while (inBounds(r, c) && traveled < maxSteps) {
        const dst = parseCode(pieceAt(board, r, c));
        if (!dst) {
          out.push({ row: r, col: c });
          traveled++;
          r += dr;
          c += dc;
          continue;
        }
        if (dst.color !== color) {
          out.push({ row: r, col: c });
        }
        break;
      }
    }
    return out;
  }

  /** Applies knight-touch, bishop-touch, and rook-touch movement hints; dedupes overlapping destinations. */
  function mergePowerMovementGrants(out, board, from, color, snapshot) {
    mergeKnightTouchGrant(out, board, from, color, snapshot);
    mergeBishopTouchGrant(out, board, from, color, snapshot);
    mergeRookTouchGrant(out, board, from, color, snapshot);
    const seen = new Set();
    let w = 0;
    for (let i = 0; i < out.length; i++) {
      const m = out[i];
      const k = `${m.row},${m.col}`;
      if (seen.has(k)) continue;
      seen.add(k);
      out[w++] = m;
    }
    out.length = w;
    return out;
  }

  /**
   * computeMoves returns pseudo-legal destinations for highlighting. En passant uses server snapshot.
   * @param {string[][]} board
   * @param {{row:number,col:number}} from logical coords
   * @param {{valid?:boolean,targetRow?:number,targetCol?:number,pawnRow?:number,pawnCol?:number}} [ep]
   * @param {{whiteKingSide?:boolean,whiteQueenSide?:boolean,blackKingSide?:boolean,blackQueenSide?:boolean}} [castlingRights]
   * @param {object} [snapshot] match snapshot for movement grants (defaults to lastSnapshot)
   * @returns {{row:number,col:number}[]}
   */
  function computeMoves(board, from, ep, castlingRights, snapshot = lastSnapshot) {
    const srcCode = pieceAt(board, from.row, from.col);
    const src = parseCode(srcCode);
    if (!src) return [];
    const out = [];
    const color = src.color;
    const type = src.type;

    if (type === "P") {
      const dir = color === "w" ? -1 : 1;
      const startRow = color === "w" ? 6 : 1;
      if (!parseCode(pieceAt(board, from.row + dir, from.col))) {
        out.push({ row: from.row + dir, col: from.col });
        if (from.row === startRow && !parseCode(pieceAt(board, from.row + 2 * dir, from.col))) {
          out.push({ row: from.row + 2 * dir, col: from.col });
        }
      }
      for (const dc of [-1, 1]) {
        const rr = from.row + dir;
        const cc = from.col + dc;
        if (!inBounds(rr, cc)) continue;
        const target = parseCode(pieceAt(board, rr, cc));
        if (target && target.color !== color) {
          out.push({ row: rr, col: cc });
          continue;
        }
        if (ep && ep.valid && rr === ep.targetRow && cc === ep.targetCol) {
          const cap = parseCode(pieceAt(board, ep.pawnRow, ep.pawnCol));
          if (cap && cap.type === "P" && cap.color !== color) {
            out.push({ row: rr, col: cc });
          }
        }
      }
      return mergePowerMovementGrants(out, board, from, color, snapshot).filter((m) => inBounds(m.row, m.col));
    }

    if (type === "N") {
      const jumps = [
        [-2, -1],
        [-2, 1],
        [-1, -2],
        [-1, 2],
        [1, -2],
        [1, 2],
        [2, -1],
        [2, 1],
      ];
      for (const [dr, dc] of jumps) pushIfValidMove(out, board, color, from.row + dr, from.col + dc);
      return mergePowerMovementGrants(out, board, from, color, snapshot);
    }
    if (type === "B") {
      slidingMoves(out, board, color, from, [
        [-1, -1],
        [-1, 1],
        [1, -1],
        [1, 1],
      ]);
      return mergePowerMovementGrants(out, board, from, color, snapshot);
    }
    if (type === "R") {
      slidingMoves(out, board, color, from, [
        [-1, 0],
        [1, 0],
        [0, -1],
        [0, 1],
      ]);
      return mergePowerMovementGrants(out, board, from, color, snapshot);
    }
    if (type === "Q") {
      slidingMoves(out, board, color, from, [
        [-1, -1],
        [-1, 1],
        [1, -1],
        [1, 1],
        [-1, 0],
        [1, 0],
        [0, -1],
        [0, 1],
      ]);
      return mergePowerMovementGrants(out, board, from, color, snapshot);
    }
    if (type === "K") {
      const opponentColor = color === "w" ? "b" : "w";
      for (let dr = -1; dr <= 1; dr++) {
        for (let dc = -1; dc <= 1; dc++) {
          if (dr === 0 && dc === 0) continue;
          const rr = from.row + dr;
          const cc = from.col + dc;
          if (!inBounds(rr, cc)) continue;
          const dst = parseCode(pieceAt(board, rr, cc));
          if (dst && dst.color === color) continue;
          if (!isSquareAttackedBy(board, rr, cc, opponentColor)) {
            out.push({ row: rr, col: cc });
          }
        }
      }
      const homeRow = color === "w" ? 7 : 0;
      if (from.row === homeRow && from.col === 4) {
        const kingSideRight = color === "w" ? !!castlingRights?.whiteKingSide : !!castlingRights?.blackKingSide;
        const queenSideRight = color === "w" ? !!castlingRights?.whiteQueenSide : !!castlingRights?.blackQueenSide;
        const rookKingSide = parseCode(pieceAt(board, homeRow, 7));
        const rookQueenSide = parseCode(pieceAt(board, homeRow, 0));
        const emptyF = !parseCode(pieceAt(board, homeRow, 5));
        const emptyG = !parseCode(pieceAt(board, homeRow, 6));
        const emptyB = !parseCode(pieceAt(board, homeRow, 1));
        const emptyC = !parseCode(pieceAt(board, homeRow, 2));
        const emptyD = !parseCode(pieceAt(board, homeRow, 3));
        const safeE = !isSquareAttackedBy(board, homeRow, 4, opponentColor);
        const safeF = !isSquareAttackedBy(board, homeRow, 5, opponentColor);
        const safeG = !isSquareAttackedBy(board, homeRow, 6, opponentColor);
        const safeD = !isSquareAttackedBy(board, homeRow, 3, opponentColor);
        const safeC = !isSquareAttackedBy(board, homeRow, 2, opponentColor);
        if (
          kingSideRight &&
          rookKingSide &&
          rookKingSide.type === "R" &&
          rookKingSide.color === color &&
          emptyF &&
          emptyG &&
          safeE &&
          safeF &&
          safeG
        ) {
          out.push({ row: homeRow, col: 6 });
        }
        if (
          queenSideRight &&
          rookQueenSide &&
          rookQueenSide.type === "R" &&
          rookQueenSide.color === color &&
          emptyB &&
          emptyC &&
          emptyD &&
          safeE &&
          safeD &&
          safeC
        ) {
          out.push({ row: homeRow, col: 2 });
        }
      }
    }
    return mergePowerMovementGrants(out, board, from, color, snapshot);
  }

  function isOwnPiece(code) {
    const p = parseCode(code);
    if (!p) return false;
    const local = playerEl.value === "A" ? "w" : "b";
    return p.color === local;
  }

  function setBar(fillEl, labelEl, cur, max) {
    if (!fillEl || !labelEl) return;
    const m = Math.max(1, max || 1);
    const pct = Math.min(100, Math.round((100 * (cur || 0)) / m));
    fillEl.style.width = `${pct}%`;
    labelEl.textContent = `${cur ?? 0}/${max ?? 0}`;
  }

  // ---------------------------------------------------------------------------
  // Playmat: previous snapshot reference for animation diffing
  // ---------------------------------------------------------------------------
  let pmPrevSnapshot = null;
  // Updated synchronously on every received snapshot (before enqueueSnapshotApply).
  // Used to detect reaction-resolve transitions at arrival time, independently of the
  // async queue which updates pmPrevSnapshot only when each closure executes.
  let prevReceivedSnapshot = null;

  /** @type {Promise<void>} */
  let snapshotApplyChain = Promise.resolve();

  /**
   * Runs snapshot-driven UI updates sequentially so turn-start counter animations cannot
   * interleave with a later state_snapshot.
   * @param {() => void | Promise<void>} fn
   */
  function enqueueSnapshotApply(fn) {
    snapshotApplyChain = snapshotApplyChain
      .then(async () => {
        try {
          await Promise.resolve(fn());
        } finally {
          renderPlayerHud(lastSnapshot);
        }
      })
      .catch((err) => {
        console.error("snapshot apply error", err);
      });
  }

  // ---------------------------------------------------------------------------
  // Playmat zone elements (cached once)
  // ---------------------------------------------------------------------------
  const pmEl = {
    deckSelf: document.getElementById("deckSelf"),
    deckOpp: document.getElementById("deckOpp"),
    deckSleeveSelf: document.getElementById("deckSleeveSelf"),
    deckSleeveOpp: document.getElementById("deckSleeveOpp"),
    deckCountSelf: document.getElementById("deckCountSelf"),
    deckCountOpp: document.getElementById("deckCountOpp"),
    drawBtn: document.getElementById("drawBtn"),
    graveyardGridSelf: document.getElementById("graveyardGridSelf"),
    graveyardGridOpp: document.getElementById("graveyardGridOpp"),
    graveyardZoneSelf: document.getElementById("graveyardSelf"),
    graveyardZoneOpp: document.getElementById("graveyardOpp"),
    banishTopSelf: document.getElementById("banishTopSelf"),
    banishTopOpp: document.getElementById("banishTopOpp"),
    ignitionCardSelf: document.getElementById("ignitionCardSelf"),
    ignitionCardOpp: document.getElementById("ignitionCardOpp"),
    ignitionCounterSelf: document.getElementById("ignitionCounterSelf"),
    ignitionCounterOpp: document.getElementById("ignitionCounterOpp"),
    cooldownCardsSelf: document.getElementById("cooldownCardsSelf"),
    cooldownCardsOpp: document.getElementById("cooldownCardsOpp"),
    handSelf: document.getElementById("handSelf"),
    handOpp: document.getElementById("handOpp"),
    matchCardPreview: document.getElementById("matchCardPreview"),
    banishSelf: document.getElementById("banishSelf"),
    banishOpp: document.getElementById("banishOpp"),
    cooldownSelf: document.getElementById("cooldownSelf"),
    cooldownOpp: document.getElementById("cooldownOpp"),
    ignitionSelf: document.getElementById("ignitionSelf"),
    ignitionOpp: document.getElementById("ignitionOpp"),
    playerActionBanner: document.getElementById("playerActionBanner"),
    playerActionHint: document.getElementById("playerActionHint"),
    mulliganBar: document.getElementById("mulliganBar"),
    mulliganHint: document.getElementById("mulliganHint"),
    mulliganTimer: document.getElementById("mulliganTimer"),
    mulliganCounts: document.getElementById("mulliganCounts"),
    mulliganConfirmBtn: document.getElementById("mulliganConfirmBtn"),
  };

  /**
   * Returns the HUD entry for seat A/B in a state snapshot.
   * @param {object|null} snap
   * @param {string} pid
   * @returns {object|undefined}
   */
  function hudForSeat(snap, pid) {
    return snap?.players?.find((p) => p.playerId === pid);
  }

  /**
   * Stable string of per-player ignition fields for snapshot diffing (animation gate).
   * @param {object|null} snap
   * @returns {string}
   */
  function ignitionHudSignature(snap) {
    const ps = snap?.players;
    if (!Array.isArray(ps)) return "";
    return ps
      .map((p) =>
        [
          p.playerId,
          p.ignitionOn ? "1" : "0",
          p.ignitionCard || "",
          String(p.ignitionTurnsRemaining ?? 0),
          p.ignitionEffectNegated ? "n" : "",
        ].join(":"),
      )
      .sort()
      .join("|");
  }

  /**
   * Stable digest of playmat-relevant state for snapshot animation deduping.
   * Ignores wall-clock / deadline fields so duplicate `state_snapshot` broadcasts
   * (e.g. paired `client_fx_release` from both clients) do not re-run flies.
   * @param {object|null} snap
   * @returns {string}
   */
  function snapshotPlaymatAnimationDigest(snap) {
    if (!snap || !Array.isArray(snap.players)) return "";
    const rw = snap.reactionWindow || {};
    const players = [...snap.players]
      .map((p) => ({
        id: p.playerId,
        hand: (p.hand || []).map((c) => c.cardId).join(","),
        ign: p.ignitionOn ? `${p.ignitionCard}:${p.ignitionTurnsRemaining ?? 0}` : "",
        cd: (p.cooldownPreview || []).map((e) => `${e.cardId}:${e.turnsRemaining}`).join(";"),
        bc: (p.banishedCards || []).length,
        hc: p.handCount,
        dco: p.deckCount,
      }))
      .sort((a, b) => String(a.id).localeCompare(String(b.id)));
    const pc = snap.pendingCapture;
    const pcKey = pc && pc.active ? `${pc.actor || ""}:${pc.fromRow},${pc.fromCol}-${pc.toRow},${pc.toCol}` : "";
    return JSON.stringify({
      t: snap.turnPlayer,
      tn: snap.turnNumber,
      b: snap.board,
      gs: snap.gameStarted,
      me: snap.matchEnded,
      rw: {
        o: !!rw.open,
        tr: rw.trigger || "",
        ac: rw.actor || "",
        ss: rw.stackSize ?? 0,
        sc: rw.stagedCardId || "",
        so: rw.stagedOwner || "",
        st: (rw.stackCards || []).map((c) => `${c.owner}:${c.cardId}`).join("|"),
      },
      pc: pcKey,
      aq: snap.activationQueueSize ?? 0,
      pe: Array.isArray(snap.pendingEffects) ? snap.pendingEffects.length : 0,
      ig: ignitionHudSignature(snap),
      players,
    });
  }

  // ---------------------------------------------------------------------------
  // Playmat: card preview hover (hover shows full card at cursor)
  // ---------------------------------------------------------------------------
  let pmPreviewCard = null;
  /** Remember description vs example per catalog id while on the match screen (same idea as deck builder). */
  const matchHandExampleMode = new Map();

  /**
   * Syncs description/example mode for every match playmat card with this catalog id (hand, ignition, cooldown, banish, hover preview).
   * @param {string} cardId
   * @param {boolean} showingExample
   */
  function syncAllMatchPowerCardMinis(cardId, showingExample) {
    if (!cardId) return;
    function applyHolder(holder) {
      if (!holder || holder.dataset.cardId !== cardId) return;
      const article = holder.querySelector(".power-card");
      if (article && typeof globalThis.setPowerCardExampleMode === "function") {
        globalThis.setPowerCardExampleMode(article, showingExample);
      }
    }
    const root = gameShellEl || document.getElementById("gameShell");
    if (root) {
      root.querySelectorAll("[data-card-id]").forEach((holder) => applyHolder(holder));
    }
  }

  function showCardPreview(cardData, anchorEl) {
    if (!pmEl.matchCardPreview || !cardData) return;
    pmEl.matchCardPreview.innerHTML = "";
    const cid = cardData.id != null ? String(cardData.id) : "";
    if (cid) {
      pmEl.matchCardPreview.dataset.cardId = cid;
    } else {
      delete pmEl.matchCardPreview.dataset.cardId;
    }
    const card = createPowerCard({
      type: cardData.type,
      name: cardData.name,
      description: cardData.description,
      example: cardData.example,
      mana: cardData.manaCost ?? cardData.mana,
      ignition: cardData.ignition,
      cooldown: cardData.cooldown,
      cardWidth: "260px",
      showExampleInitially: Boolean(cid && matchHandExampleMode.get(cid) === true),
      onExampleToggle: (showing) => {
        if (cid) matchHandExampleMode.set(cid, showing);
        syncAllMatchPowerCardMinis(cid, showing);
      },
    });
    pmEl.matchCardPreview.appendChild(card);
    pmEl.matchCardPreview.classList.remove("hidden");
    pmPreviewCard = anchorEl;
    // Double rAF so the card renders and offsetWidth/offsetHeight are available.
    requestAnimationFrame(() => {
      requestAnimationFrame(() => positionCardPreview(anchorEl));
    });
  }

  /**
   * Shows a stacked multi-card preview for piece effects. Each entry is {cardData, turnsRemaining}.
   * When turnsRemaining is null the per-card turn badge is omitted.
   */
  function showPieceEffectsPreview(effects, anchorEl) {
    if (!pmEl.matchCardPreview || !effects || effects.length === 0) return;
    pmEl.matchCardPreview.innerHTML = "";
    delete pmEl.matchCardPreview.dataset.cardId;

    for (const { cardData, turnsRemaining } of effects) {
      if (!cardData) continue;
      const cid = cardData.id != null ? String(cardData.id) : "";
      const wrapper = document.createElement("div");
      wrapper.style.position = "relative";
      wrapper.style.display = "inline-block";
      const card = createPowerCard({
        type: cardData.type,
        name: cardData.name,
        description: cardData.description,
        example: cardData.example,
        mana: cardData.manaCost ?? cardData.mana,
        ignition: cardData.ignition,
        cooldown: cardData.cooldown,
        cardWidth: "260px",
        showExampleInitially: Boolean(cid && matchHandExampleMode.get(cid) === true),
        onExampleToggle: (showing) => {
          if (cid) matchHandExampleMode.set(cid, showing);
          syncAllMatchPowerCardMinis(cid, showing);
        },
      });
      wrapper.appendChild(card);
      if (turnsRemaining != null) {
        const badge = document.createElement("span");
        badge.className = "pm-card-turns-badge";
        badge.textContent = `${turnsRemaining}t`;
        wrapper.appendChild(badge);
      }
      pmEl.matchCardPreview.appendChild(wrapper);
    }

    pmEl.matchCardPreview.classList.remove("hidden");
    pmPreviewCard = anchorEl;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => positionCardPreview(anchorEl));
    });
  }

  function positionCardPreview(anchorEl) {
    if (!pmEl.matchCardPreview || !anchorEl) return;
    const wrap = pmEl.matchCardPreview;
    const pad = 10;
    const w = wrap.offsetWidth;
    const h = wrap.offsetHeight;
    const rect = anchorEl.getBoundingClientRect();
    // Center horizontally over the anchor; prefer above, fall back to below.
    let left = rect.left + rect.width / 2 - w / 2;
    let top = rect.top - h - 12;
    left = Math.max(pad, Math.min(left, window.innerWidth - w - pad));
    if (top < pad) {
      top = rect.bottom + 10;
    }
    if (top + h > window.innerHeight - pad) {
      top = Math.max(pad, window.innerHeight - h - pad);
    }
    wrap.style.left = `${Math.round(left)}px`;
    wrap.style.top = `${Math.round(top)}px`;
  }

  function hideCardPreview() {
    if (!pmEl.matchCardPreview) return;
    pmEl.matchCardPreview.classList.add("hidden");
    pmEl.matchCardPreview.innerHTML = "";
    delete pmEl.matchCardPreview.dataset.cardId;
    pmPreviewCard = null;
  }

  function getCardDef(cardId) {
    const catalog = getLocalizedCardCatalog(locale);
    return catalog.find((c) => c.id === cardId) || null;
  }

  /** Builds the payload expected by {@link showCardPreview} from a localized catalog row. */
  function cardDataFromCatalogRow(def) {
    if (!def) return null;
    return {
      id: def.id,
      type: def.type,
      name: def.name,
      description: def.description,
      example: def.example,
      mana: def.mana,
      ignition: def.ignition,
      cooldown: def.cooldown,
    };
  }

  /**
   * @returns {boolean} Whether {@link applyPlaymatUiTestOverlay} should run (see {@link PLAYMAT_UI_TEST_OVERLAY}).
   */
  function isPlaymatUiTestEnabled() {
    return PLAYMAT_UI_TEST_OVERLAY && gameShellEl && !gameShellEl.classList.contains("hidden");
  }

  /**
   * Deep-clones the snapshot and fills both players with 5 hand cards (face-up for self,
   * handCount for opponent), 3 banished, and 6 cooldown entries with random catalog ids.
   * Does not change server state; render-only.
   * @param {object} snapshot
   * @returns {object}
   */
  function applyPlaymatUiTestOverlay(snapshot) {
    const catalog = getLocalizedCardCatalog(locale);
    const ids = catalog.map((c) => c.id).filter(Boolean);
    if (ids.length === 0) return snapshot;
    const snap = JSON.parse(JSON.stringify(snapshot));
    const localPID = playerEl.value;
    const self = snap.players.find((p) => p.playerId === localPID);
    const opp = snap.players.find((p) => p.playerId !== localPID);
    if (!self || !opp) return snapshot;

    function defFor(cardId) {
      return catalog.find((c) => c.id === cardId) || null;
    }
    function cardEntry(cardId) {
      const def = defFor(cardId);
      return {
        cardId,
        manaCost: def ? def.mana : 0,
        ignition: def ? def.ignition : 0,
        cooldown: def ? def.cooldown : 0,
      };
    }
    function pickRandomIds(n) {
      const out = [];
      for (let i = 0; i < n; i++) {
        out.push(ids[Math.floor(Math.random() * ids.length)]);
      }
      return out;
    }

    const handSelf = pickRandomIds(5);
    self.hand = handSelf.map((id) => cardEntry(id));
    self.handCount = 5;
    opp.handCount = 5;
    self.banishedCards = pickRandomIds(3).map((id) => cardEntry(id));
    opp.banishedCards = pickRandomIds(3).map((id) => cardEntry(id));
    self.cooldownPreview = pickRandomIds(6).map((id, i) => ({
      ...cardEntry(id),
      turnsRemaining: 1 + (i % 5),
    }));
    opp.cooldownPreview = pickRandomIds(6).map((id, i) => ({
      ...cardEntry(id),
      turnsRemaining: 1 + (i % 5),
    }));
    self.cooldownHiddenCount = 0;
    opp.cooldownHiddenCount = 0;
    self.cooldownCount = 6;
    opp.cooldownCount = 6;
    // 15 captured pieces each: Q, 2×R, 2×B, 2×N, 8×P — distinct colors for self vs opp
    self.graveyardPieces = buildFifteenGraveyardTestCodes("w");
    opp.graveyardPieces = buildFifteenGraveyardTestCodes("b");
    return snap;
  }

  /**
   * Builds 15 piece codes (max non-king material) for graveyard UI tests: Q>R>B>N>P order in flat list
   * (renderer groups by type). Color prefix is `w` or `b`.
   * @param {string} colorPrefix
   * @returns {string[]}
   */
  function buildFifteenGraveyardTestCodes(colorPrefix) {
    const types = ["Q", "R", "R", "B", "B", "N", "N", "P", "P", "P", "P", "P", "P", "P", "P"];
    return types.map((t) => `${colorPrefix}${t}`);
  }

  /**
   * Sort key for graveyard piece type (matches server graveyardPieceImportance): Q > R > B > N > P.
   * @param {string} type
   * @returns {number}
   */
  function graveyardTypeRank(type) {
    switch (type) {
      case "Q":
        return 0;
      case "R":
        return 1;
      case "B":
        return 2;
      case "N":
        return 3;
      case "P":
        return 4;
      default:
        return 5;
    }
  }

  // ---------------------------------------------------------------------------
  // Playmat: sleeve URL helper
  // ---------------------------------------------------------------------------
  function sleeveUrl(color) {
    const valid = ["blue", "green", "pink", "red"];
    const c = valid.includes(color) ? color : "blue";
    return `/public/sleeves/${c}Sleeve.png`;
  }

  // ---------------------------------------------------------------------------
  // Playmat: render all zones from snapshot
  // ---------------------------------------------------------------------------

  // ---------------------------------------------------------------------------
  // Playmat: animation utilities
  // ---------------------------------------------------------------------------

  /**
   * @param {number} ms
   * @returns {Promise<void>}
   */
  function sleep(ms) {
    return new Promise((r) => setTimeout(r, ms));
  }

  /**
   * Escapes a card id for use in a CSS attribute selector.
   * @param {string} id
   * @returns {string}
   */
  function escapeCardIdForSelector(id) {
    if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
      return CSS.escape(String(id));
    }
    return String(id).replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  }

  /**
   * Vertical odometer-style roll from fromText to toText; total duration totalMs.
   * @param {HTMLElement | null} el
   * @param {string} fromText
   * @param {string} toText
   * @param {number} [totalMs]
   * @returns {Promise<void>}
   */
  async function odometerFlip(el, fromText, toText, totalMs = 700) {
    if (!el || fromText === toText) return;
    const liftWrap = el.closest(".pm-cooldown-card-wrap");
    if (liftWrap) liftWrap.classList.add("pm-cooldown-wrap--odometer-active");
    const t0 = totalMs * 0.25;
    const t1 = totalMs * 0.5;
    const t2 = totalMs * 0.25;
    try {
      el.classList.add("pm-odometer-highlight");
      await el.animate([{ transform: "scale(1)" }, { transform: "scale(1.25)" }], {
        duration: t0,
        easing: "ease-out",
        fill: "forwards",
      }).finished;

      const viewport = document.createElement("span");
      viewport.className = "pm-odometer-viewport";
      const track = document.createElement("span");
      track.className = "pm-odometer-track";
      const line0 = document.createElement("span");
      line0.className = "pm-odometer-line";
      line0.textContent = fromText;
      const line1 = document.createElement("span");
      line1.className = "pm-odometer-line";
      line1.textContent = toText;
      track.appendChild(line0);
      track.appendChild(line1);
      viewport.appendChild(track);
      el.innerHTML = "";
      el.appendChild(viewport);

      const h = line0.getBoundingClientRect().height;
      track.style.transform = "translateY(0)";
      await track.animate([{ transform: "translateY(0)" }, { transform: `translateY(-${h}px)` }], {
        duration: t1,
        easing: "cubic-bezier(0.4,0,0.2,1)",
        fill: "forwards",
      }).finished;

      el.textContent = toText;
      el.style.transform = "scale(1.25)";
      el.classList.remove("pm-odometer-highlight");
      await el.animate([{ transform: "scale(1.25)" }, { transform: "scale(1)" }], {
        duration: t2,
        easing: "ease-out",
        fill: "forwards",
      }).finished;
      el.style.transform = "";
    } finally {
      if (liftWrap) liftWrap.classList.remove("pm-cooldown-wrap--odometer-active");
    }
  }

  /**
   * Runs cooldown odometer tasks (excluding the arriving-from-ignition card) for the turn starter.
   * @param {object} ctx
   * @param {{ cardId: string, from: string, to: string }[]} ctx.cooldownTasks
   * @param {HTMLElement | null | undefined} ctx.cdContainer
   * @param {object} ctx.nextSnap
   * @param {string} ctx.localPID
   */
  async function runTurnStartCooldownOdometersLoop(ctx) {
    const { cooldownTasks, cdContainer, nextSnap, localPID } = ctx;
    const delayMs = 200;
    const animMs = 700;
    for (let i = 0; i < cooldownTasks.length; i++) {
      const task = cooldownTasks[i];
      const taskWrap = cdContainer?.querySelector(`[data-card-id="${escapeCardIdForSelector(String(task.cardId))}"]`);
      const turnsEl = taskWrap?.querySelector(".pm-cooldown-turns");
      if (turnsEl) {
        await odometerFlip(turnsEl, task.from, task.to, animMs);
      }
      if (task.to === "0t") {
        taskWrap?.remove();
        const localSelf = nextSnap.players?.find((p) => p.playerId === localPID);
        let localOpp = nextSnap.players?.find((p) => p.playerId !== localPID);
        if (!localOpp) localOpp = emptyOppPlaymatStub(localPID);
        if (localSelf) {
          renderBanishZone(localSelf, localOpp);
          renderDeckZone(localSelf, localOpp);
        }
      }
      if (i < cooldownTasks.length - 1) await sleep(delayMs);
    }
  }

  /**
   * After activate_card outcome (payload success/fail): place resolved ignition card in cooldown, then tick other cooldown odometers.
   * @param {object} ctx
   */
  async function applyIgnitionResolveThenCooldownOdometers(ctx) {
    const { nextSnap, localPID, cdContainer, prevCD, nextCD, arrivingIgnitionCardId, cooldownTasks } = ctx;
    const arrivingEntry = nextCD.find((e) => e.cardId === arrivingIgnitionCardId);
    const intermediateList = [
      ...prevCD.filter((e) => e.cardId !== arrivingIgnitionCardId),
      ...(arrivingEntry ? [arrivingEntry] : []),
    ];
    if (cdContainer) {
      renderCooldownList(cdContainer, sortCooldownEntriesForDisplay(intermediateList));
    }
    if (!nextCD.find((e) => e.cardId === arrivingIgnitionCardId)) {
      const localSelf = nextSnap.players?.find((p) => p.playerId === localPID);
      let localOpp = nextSnap.players?.find((p) => p.playerId !== localPID);
      if (!localOpp) localOpp = emptyOppPlaymatStub(localPID);
      if (localSelf) {
        renderBanishZone(localSelf, localOpp);
        renderDeckZone(localSelf, localOpp);
      }
    }
    await runTurnStartCooldownOdometersLoop(ctx);
    if (cdContainer) {
      renderCooldownList(cdContainer, sortCooldownEntriesForDisplay(nextCD));
    }
  }

  /**
   * Animates ignition counter at turn start; optionally defers cooldown odometers until after activate_card.
   * When deferCooldownUntilAfterActivate: keeps the card in the ignition slot until runEffectActivationClientSequence
   * finishes the effect glow, then moves to cooldown and runs other recharge odometers.
   * @param {object | null} prevSnap
   * @param {object} nextSnap
   * @param {{ deferCooldownUntilAfterActivate?: boolean }} [options]
   * @returns {Promise<object | null>} deferral context for flushPendingActivationFx, or null
   */
  async function runTurnStartResourceSequence(prevSnap, nextSnap, options) {
    if (!prevSnap || !nextSnap) return null;
    const localPID = playerEl.value;
    const turnStarter = nextSnap.turnPlayer;
    if (!turnStarter || prevSnap.turnPlayer === nextSnap.turnPlayer) return null;

    const deferCooldownUntilAfterActivate = !!options?.deferCooldownUntilAfterActivate;
    const animMs = 700;

    const prevTurnStarterHud = hudForSeat(prevSnap, turnStarter);
    const nextTurnStarterHud = hudForSeat(nextSnap, turnStarter);
    const ignitionResolvedForTurnStarter = !!prevTurnStarterHud?.ignitionOn && !nextTurnStarterHud?.ignitionOn;
    const arrivingIgnitionCardId = ignitionResolvedForTurnStarter ? String(prevTurnStarterHud?.ignitionCard || "") : "";

    /** @type {{ el: HTMLElement, from: string, to: string } | null} */
    let ignitionOdometer = null;
    if (prevTurnStarterHud?.ignitionOn) {
      const prevTurns = prevTurnStarterHud.ignitionTurnsRemaining ?? 0;
      const stillOn = !!nextTurnStarterHud?.ignitionOn;
      const nextTurns = stillOn ? (nextTurnStarterHud.ignitionTurnsRemaining ?? 0) : 0;
      if (prevTurns > nextTurns) {
        const counterEl = turnStarter === localPID ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;
        if (counterEl) {
          ignitionOdometer = {
            el: counterEl,
            from: `${prevTurns}t`,
            to: `${nextTurns}t`,
          };
        }
      }
    }

    /** @type {{ cardId: string, from: string, to: string }[]} */
    const cooldownTasks = [];
    const prevStarter = prevSnap.players?.find((p) => p.playerId === turnStarter);
    const nextStarter = nextSnap.players?.find((p) => p.playerId === turnStarter);
    const cdContainer = turnStarter === localPID ? pmEl.cooldownCardsSelf : pmEl.cooldownCardsOpp;
    let prevCD = [],
      nextCD = [];
    if (prevStarter && nextStarter) {
      prevCD = sortCooldownEntriesForDisplay(prevStarter.cooldownPreview || []);
      nextCD = sortCooldownEntriesForDisplay(nextStarter.cooldownPreview || []);
      for (const nEntry of nextCD) {
        if (arrivingIgnitionCardId && nEntry.cardId === arrivingIgnitionCardId) continue;
        const pEntry = prevCD.find((p) => p.cardId === nEntry.cardId);
        if (!pEntry || nEntry.turnsRemaining >= pEntry.turnsRemaining) continue;
        cooldownTasks.push({
          cardId: nEntry.cardId,
          from: `${pEntry.turnsRemaining}t`,
          to: `${nEntry.turnsRemaining}t`,
        });
      }
      for (const pEntry of prevCD) {
        if (arrivingIgnitionCardId && pEntry.cardId === arrivingIgnitionCardId) continue;
        if (nextCD.find((n) => n.cardId === pEntry.cardId)) continue;
        if (pEntry.turnsRemaining <= 0) continue;
        cooldownTasks.push({
          cardId: pEntry.cardId,
          from: `${pEntry.turnsRemaining}t`,
          to: "0t",
        });
      }
    }

    /** @type {object | null} */
    let deferral = null;

    if (ignitionOdometer) {
      await odometerFlip(ignitionOdometer.el, ignitionOdometer.from, ignitionOdometer.to, animMs);
      const resolved =
        !!prevTurnStarterHud?.ignitionOn &&
        !nextTurnStarterHud?.ignitionOn &&
        ignitionOdometer.to === "0t" &&
        !!arrivingIgnitionCardId;
      if (resolved) {
        if (deferCooldownUntilAfterActivate) {
          deferral = {
            deferCooldownAfterActivation: true,
            turnStarter,
            arrivingIgnitionCardId,
            cooldownTasks,
            cdContainer,
            nextSnap,
            prevCD,
            nextCD,
            localPID,
          };
        } else if (cdContainer) {
          renderIgnitionZone(nextSnap);
          const arrivingEntry = nextCD.find((e) => e.cardId === arrivingIgnitionCardId);
          const intermediateList = [
            ...prevCD.filter((e) => e.cardId !== arrivingIgnitionCardId),
            ...(arrivingEntry ? [arrivingEntry] : []),
          ];
          renderCooldownList(cdContainer, sortCooldownEntriesForDisplay(intermediateList));
          if (!nextCD.find((e) => e.cardId === arrivingIgnitionCardId)) {
            const localSelf = nextSnap.players?.find((p) => p.playerId === localPID);
            let localOpp = nextSnap.players?.find((p) => p.playerId !== localPID);
            if (!localOpp) localOpp = emptyOppPlaymatStub(localPID);
            if (localSelf) {
              renderBanishZone(localSelf, localOpp);
              renderDeckZone(localSelf, localOpp);
            }
          }
        }
      } else if (nextTurnStarterHud?.ignitionOn) {
        renderIgnitionZone(nextSnap);
      }
    }

    if (!deferral) {
      await runTurnStartCooldownOdometersLoop({
        cooldownTasks,
        cdContainer,
        nextSnap,
        localPID,
      });
      if (cdContainer && (prevCD.length > 0 || nextCD.length > 0)) {
        renderCooldownList(cdContainer, sortCooldownEntriesForDisplay(nextCD));
      }
    }

    return deferral;
  }

  /**
   * Runs glow animations when transitioning from prevSnap to nextSnap.
   * @param {boolean} [reactionStackJustCleared] when true, skip fallback ignition glow (activate_card already ran)
   */
  function runSnapshotAnimations(prevSnap, nextSnap, reactionStackJustCleared) {
    if (!prevSnap || !nextSnap) return;
    const localPID = playerEl.value;

    const prevSelf = prevSnap.players?.find((p) => p.playerId === localPID);
    const nextSelf = nextSnap.players?.find((p) => p.playerId === localPID);
    const prevOpp = prevSnap.players?.find((p) => p.playerId !== localPID);
    const nextOpp = nextSnap.players?.find((p) => p.playerId !== localPID);

    if (!prevSelf || !nextSelf) return;

    // 3a. Card placed in ignition: was not occupied, now is → brief glow on that seat's ignition zone.
    for (const pid of ["A", "B"]) {
      const pPrev = hudForSeat(prevSnap, pid);
      const pNext = hudForSeat(nextSnap, pid);
      if (pPrev && pNext && !pPrev.ignitionOn && pNext.ignitionOn) {
        const slotEl = pid === localPID ? pmEl.ignitionSelf : pmEl.ignitionOpp;
        if (slotEl) {
          slotEl.classList.remove("pm-ignition-activating");
          void slotEl.offsetWidth;
          slotEl.classList.add("pm-ignition-activating");
          setTimeout(() => slotEl.classList.remove("pm-ignition-activating"), 650);
        }
      }
    }

    // 3c. Fallback: ignition=0 card went directly to cooldown (no ignition slot visible in either snapshot).
    // Skip when a reaction chain just resolved — activate_card already fired the glow.
    if (!reactionStackJustCleared && !prevSelf?.ignitionOn && !nextSelf?.ignitionOn) {
      const prevSelfCD = prevSelf?.cooldownPreview || [];
      const nextSelfCD = nextSelf?.cooldownPreview || [];
      const newSelfEntry = nextSelfCD.find((e) => !prevSelfCD.some((p) => p.cardId === e.cardId));
      if (newSelfEntry && (getCardDef(newSelfEntry.cardId)?.ignition ?? -1) === 0) {
        const slotEl = pmEl.ignitionSelf;
        if (slotEl) {
          slotEl.classList.remove("pm-ignition-activating");
          void slotEl.offsetWidth;
          slotEl.classList.add("pm-ignition-activating");
          setTimeout(() => slotEl.classList.remove("pm-ignition-activating"), 650);
        }
      }
    }
    if (!reactionStackJustCleared && nextOpp && prevOpp && !prevOpp?.ignitionOn && !nextOpp?.ignitionOn) {
      const prevOppCD = prevOpp?.cooldownPreview || [];
      const nextOppCD = nextOpp?.cooldownPreview || [];
      const newOppEntry = nextOppCD.find((e) => !prevOppCD.some((p) => p.cardId === e.cardId));
      if (newOppEntry && (getCardDef(newOppEntry.cardId)?.ignition ?? -1) === 0) {
        const slotEl = pmEl.ignitionOpp;
        if (slotEl) {
          slotEl.classList.remove("pm-ignition-activating");
          void slotEl.offsetWidth;
          slotEl.classList.add("pm-ignition-activating");
          setTimeout(() => slotEl.classList.remove("pm-ignition-activating"), 650);
        }
      }
    }
  }

  /**
   * Cooldown display order: soonest to resolve first (left), tie-break by card id.
   * @param {Array<{ cardId: string, turnsRemaining: number }>} list
   * @returns {Array<{ cardId: string, turnsRemaining: number }>}
   */
  function sortCooldownEntriesForDisplay(list) {
    if (!list || list.length === 0) return [];
    return [...list].sort((a, b) => {
      const ta = Number(a.turnsRemaining) || 0;
      const tb = Number(b.turnsRemaining) || 0;
      if (ta !== tb) return ta - tb;
      return String(a.cardId).localeCompare(String(b.cardId));
    });
  }

  /**
   * Minimal opponent slice when the snapshot has only one player entry (defensive).
   * @param {string} localPID
   * @returns {object}
   */
  function emptyOppPlaymatStub(localPID) {
    const oid = localPID === "A" ? "B" : "A";
    return {
      playerId: oid,
      handCount: 0,
      deckCount: 0,
      sleeveColor: "",
      banishedCards: [],
      graveyardPieces: [],
      cooldownPreview: [],
      cooldownHiddenCount: 0,
    };
  }

  /** @param {object} snapshot */
  function renderPlaymat(snapshot) {
    if (!snapshot || !snapshot.players) return;
    clearDanglingHandDragCards();
    const view = isPlaymatUiTestEnabled() ? applyPlaymatUiTestOverlay(snapshot) : snapshot;
    const localPID = String(
      (snapshot.viewerPlayerId &&
        (snapshot.viewerPlayerId === "A" || snapshot.viewerPlayerId === "B") &&
        snapshot.viewerPlayerId) ||
        playerEl.value ||
        "",
    ).trim();
    if (localPID !== "A" && localPID !== "B") return;
    const self = view.players.find((p) => p.playerId === localPID);
    let opp = view.players.find((p) => p.playerId !== localPID);
    if (!self) return;
    if (!opp) opp = emptyOppPlaymatStub(localPID);

    renderDeckZone(self, opp);
    renderGraveyardZone(self, opp);
    renderBanishZone(self, opp);
    renderIgnitionZone(snapshot);
    renderCooldownZone(self, opp);
    renderHandZone(self, opp);
    renderPlayerActionBanner(snapshot);
    updateDrawButton(snapshot, self);
    renderMulliganBar(snapshot);
  }

  /**
   * Shows mulligan instructions, public return counts, and confirm for the local player when the opening phase is active.
   * @param {object} snapshot
   */
  function clearMulliganCountdownTimer() {
    if (mulliganUiTimerId) {
      clearInterval(mulliganUiTimerId);
      mulliganUiTimerId = null;
    }
  }

  function renderMulliganBar(snapshot) {
    const bar = pmEl.mulliganBar;
    const hint = pmEl.mulliganHint;
    const timerEl = pmEl.mulliganTimer;
    const countsEl = pmEl.mulliganCounts;
    const btn = pmEl.mulliganConfirmBtn;
    if (!bar || !hint || !countsEl || !btn) return;

    const active = !!snapshot.mulliganPhaseActive;
    bar.classList.toggle("hidden", !active);
    if (!active) {
      clearMulliganCountdownTimer();
      if (timerEl) timerEl.textContent = "";
      mulliganPick.clear();
      return;
    }

    hint.textContent = t("mulliganHint");
    if (timerEl) {
      clearMulliganCountdownTimer();
      const deadline = snapshot.mulliganDeadlineUnixMs;
      if (deadline) {
        const tick = () => {
          const snap = lastSnapshot;
          if (!snap?.mulliganPhaseActive || !snap.mulliganDeadlineUnixMs) {
            clearMulliganCountdownTimer();
            timerEl.textContent = "";
            return;
          }
          const left = Math.max(0, Math.ceil((snap.mulliganDeadlineUnixMs - Date.now()) / 1000));
          timerEl.textContent = t("mulliganAutoIn", { s: left });
        };
        tick();
        mulliganUiTimerId = setInterval(tick, 250);
      } else {
        timerEl.textContent = "";
      }
    }
    const mr = snapshot.mulliganReturned || {};
    const fmt = (v) => (v === undefined || v < 0 ? t("mulliganPending") : String(v));
    countsEl.textContent = t("mulliganLine", { w: fmt(mr.A), b: fmt(mr.B) });

    const my = playerEl.value;
    const myDone = mr[my] !== undefined && mr[my] >= 0;
    if (myDone) {
      btn.disabled = true;
      btn.textContent = t("mulliganWaitingOpp");
    } else {
      btn.disabled = false;
      btn.textContent = t("mulliganConfirm");
    }
  }

  if (pmEl.mulliganConfirmBtn) {
    pmEl.mulliganConfirmBtn.addEventListener("click", () => {
      if (!lastSnapshot?.mulliganPhaseActive) return;
      const my = playerEl.value;
      if ((lastSnapshot.mulliganReturned || {})[my] >= 0) return;
      const indices = [...mulliganPick].sort((a, b) => a - b);
      send("confirm_mulligan", { handIndices: indices });
      mulliganPick.clear();
    });
  }

  function renderDeckZone(self, opp) {
    if (pmEl.deckCountSelf) pmEl.deckCountSelf.textContent = self.deckCount ?? "—";
    if (pmEl.deckCountOpp) pmEl.deckCountOpp.textContent = opp.deckCount ?? "—";
    if (pmEl.deckSleeveSelf) {
      pmEl.deckSleeveSelf.style.backgroundImage = `url('${sleeveUrl(self.sleeveColor || "blue")}')`;
    }
    if (pmEl.deckSleeveOpp) {
      pmEl.deckSleeveOpp.style.backgroundImage = `url('${sleeveUrl(opp.sleeveColor || "blue")}')`;
    }
  }

  /**
   * Renders capture-zone thumbnails. DOM ids use graveyard*; when the local player is B,
   * `.board-wrap-perspective-b` reverses the column so the physical stacks swap screen position —
   * we map self/opp pieces and own/opp styling to screen top/bottom.
   * @param {object} self Local player snapshot slice
   * @param {object} opp Opponent snapshot slice
   */
  function renderGraveyardZone(self, opp) {
    const swapForPerspectiveB = playerEl.value === "B";
    const zs = pmEl.graveyardZoneSelf;
    const zo = pmEl.graveyardZoneOpp;

    if (swapForPerspectiveB) {
      renderGraveyardGrid(pmEl.graveyardGridSelf, opp.graveyardPieces || [], "right");
      renderGraveyardGrid(pmEl.graveyardGridOpp, self.graveyardPieces || [], "left");
      zs?.classList.remove("pm-graveyard--own");
      zs?.classList.add("pm-graveyard--opp");
      zo?.classList.remove("pm-graveyard--opp");
      zo?.classList.add("pm-graveyard--own");
      zs?.setAttribute("aria-label", t("zoneCaptureAriaOpponent"));
      zo?.setAttribute("aria-label", t("zoneCaptureAriaYour"));
    } else {
      renderGraveyardGrid(pmEl.graveyardGridSelf, self.graveyardPieces || [], "left");
      renderGraveyardGrid(pmEl.graveyardGridOpp, opp.graveyardPieces || [], "right");
      zs?.classList.remove("pm-graveyard--opp");
      zs?.classList.add("pm-graveyard--own");
      zo?.classList.remove("pm-graveyard--own");
      zo?.classList.add("pm-graveyard--opp");
      zs?.setAttribute("aria-label", t("zoneCaptureAriaYour"));
      zo?.setAttribute("aria-label", t("zoneCaptureAriaOpponent"));
    }
  }

  /**
   * Renders capture thumbnails in groups by piece type (Q>R>B>N>P), stacked horizontally
   * within each group; `align` is own=left, opponent=right.
   * @param {HTMLElement | null} container
   * @param {string[]} pieces
   * @param {'left' | 'right'} align
   */
  function renderGraveyardGrid(container, pieces, align) {
    if (!container) return;
    container.innerHTML = "";
    container.classList.remove("pm-graveyard-grid--align-left", "pm-graveyard-grid--align-right");
    container.classList.toggle("pm-graveyard-grid--align-left", align === "left");
    container.classList.toggle("pm-graveyard-grid--align-right", align === "right");

    const list = pieces || [];
    if (list.length === 0) return;

    /** @type {Map<string, string[]>} */
    const byType = new Map();
    for (const code of list) {
      if (!code || code.length < 2) continue;
      const t = code[1];
      if (!byType.has(t)) byType.set(t, []);
      byType.get(t).push(code);
    }

    const typesOrdered = [...byType.keys()].sort((a, b) => graveyardTypeRank(a) - graveyardTypeRank(b));

    for (const t of typesOrdered) {
      const codes = byType.get(t);
      if (!codes || codes.length === 0) continue;

      const group = document.createElement("div");
      group.className = "pm-graveyard-group";
      const n = codes.length;
      group.style.setProperty("--gy-n", String(n));

      for (let i = 0; i < codes.length; i++) {
        const code = codes[i];
        const url = pieceImageURL(code);
        if (!url) continue;
        const img = document.createElement("img");
        img.src = url;
        img.className = "pm-graveyard-piece";
        img.alt = code;
        img.style.setProperty("--i", String(i));
        group.appendChild(img);
      }
      container.appendChild(group);
    }
  }

  function renderBanishZone(self, opp) {
    renderBanishTop(pmEl.banishTopSelf, self.banishedCards || [], self.sleeveColor);
    renderBanishTop(pmEl.banishTopOpp, opp.banishedCards || [], opp.sleeveColor);
  }

  function renderBanishTop(container, cards, sleeve) {
    if (!container) return;
    container.innerHTML = "";
    if (cards.length === 0) return;
    const list = cards || [];
    const n = Math.max(1, list.length);
    container.style.setProperty("--cd-stack-n", String(n));

    const stack = document.createElement("div");
    stack.className = "pm-banish-stack";
    container.appendChild(stack);

    for (let i = 0; i < list.length; i++) {
      const entry = list[i];
      const wrap = document.createElement("div");
      wrap.className = "pm-banish-card-wrap";
      wrap.style.setProperty("--i", String(i));
      const def = getCardDef(entry.cardId);
      if (def) {
        const cid = String(entry.cardId);
        wrap.dataset.cardId = cid;
        const card = createPowerCard({
          type: def.type,
          name: def.name,
          description: def.description,
          example: def.example,
          mana: def.manaCost ?? entry.manaCost ?? def.mana,
          ignition: def.ignition,
          cooldown: def.cooldown,
          cardWidth: "220px",
          showExampleInitially: matchHandExampleMode.get(cid) === true,
          onExampleToggle: (showing) => {
            matchHandExampleMode.set(cid, showing);
            syncAllMatchPowerCardMinis(cid, showing);
          },
        });
        wrap.appendChild(card);
        attachCardHover(wrap, { ...def, id: cid, manaCost: def.mana });
      } else {
        const fb = document.createElement("div");
        fb.className = "pm-sleeve-card";
        fb.style.backgroundImage = `url('${sleeveUrl(sleeve)}')`;
        wrap.appendChild(fb);
      }
      stack.appendChild(wrap);
    }
  }

  /**
   * Ignition card mount for the reaction stack top: always that seat's zone (never the opponent's).
   * When that zone already shows a base ignition card, {@link renderReactionStagedIfAny} stacks the staged card as an overlay.
   */
  function pickReactionStagedSlot(snapshot, localPID, stagedOwner) {
    if (!snapshot || !localPID || !stagedOwner) return null;
    return stagedOwner === localPID ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
  }

  /**
   * Returns the ignition card wrap element used to show a given stack card during resolve animations.
   * @param {object} snapshot
   * @param {string} localPID
   * @param {string} cardId
   * @param {string} cardOwner seat "A" | "B"
   * @returns {HTMLElement | null}
   */
  function reactionStackCardWrap(snapshot, localPID, cardId, cardOwner) {
    const fakeSnap = {
      ...snapshot,
      reactionWindow: {
        ...(snapshot.reactionWindow || {}),
        open: true,
        stagedCardId: String(cardId),
        stagedOwner: String(cardOwner),
      },
    };
    let wrap = pickReactionStagedSlot(fakeSnap, localPID, cardOwner);
    if (!wrap) {
      wrap = cardOwner === localPID ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
    }
    const stagedOv = wrap?.querySelector?.(".pm-ignition-staged-overlay");
    if (stagedOv) {
      wrap = stagedOv;
    }
    return wrap || null;
  }

  /**
   * Rim color for effect activation (ignite counter hit 0): type × success/fail.
   * Power→green, Continuous→blue, Retribution→red, Counter→pink, Disruption→orange (muted on fail).
   * @param {string} typeLower
   * @param {boolean} success
   * @returns {string}
   */
  function effectActivationGlowColor(typeLower, success) {
    const a = success ? 0.92 : 0.62;
    switch (typeLower) {
      case "power":
        return `rgba(85, 210, 130, ${a})`;
      case "continuous":
        return `rgba(110, 175, 255, ${a})`;
      case "retribution":
        return `rgba(220, 85, 105, ${a})`;
      case "counter":
        return `rgba(255, 115, 190, ${a})`;
      case "disruption":
        return `rgba(255, 140, 50, ${a})`;
      default:
        return `rgba(200, 210, 230, ${success ? 0.8 : 0.55})`;
    }
  }

  /**
   * Glow when the server reports effect activation (activate_card) after ignition reaches 0.
   * @param {HTMLElement | null} wrapEl
   * @param {string} typeLower
   * @param {boolean} success
   * @returns {Promise<void>}
   */
  /**
   * After a failed activation glow: three B/W chroma pulses, hold ~1s on the last, then restore.
   * @param {HTMLElement | null} cardVisualEl usually `.power-card` inside the ignition mount
   * @returns {Promise<void>}
   */
  async function playNegationChromaSequence(cardVisualEl) {
    if (!(cardVisualEl instanceof HTMLElement)) return;
    const bw = "grayscale(1) contrast(1.4) brightness(1.08)";
    const flashMs = 170;
    const gapMs = 150;
    cardVisualEl.style.willChange = "filter";
    for (let i = 0; i < 3; i++) {
      cardVisualEl.style.transition = `filter ${flashMs}ms ease`;
      cardVisualEl.style.filter = bw;
      await new Promise((r) => setTimeout(r, flashMs));
      if (i < 2) {
        cardVisualEl.style.filter = "none";
        await new Promise((r) => setTimeout(r, gapMs));
      }
    }
    await new Promise((r) => setTimeout(r, 1000));
    cardVisualEl.style.filter = "none";
    cardVisualEl.style.transition = "";
    cardVisualEl.style.willChange = "";
  }

  /**
   * @param {HTMLElement | null} wrap ignition mount or staged overlay
   * @returns {HTMLElement | null}
   */
  function ignitionActivationCardVisual(wrap) {
    if (!wrap) return null;
    return wrap.querySelector(".power-card") || wrap;
  }

  async function playEffectActivationGlow(wrapEl, typeLower, success) {
    if (!wrapEl) return;
    const rect = wrapEl.getBoundingClientRect();
    if (!rect || rect.width < 2 || rect.height < 2) return;
    const pad = 12;
    const el = document.createElement("div");
    el.className = "pm-effect-activation-glow-overlay";
    el.setAttribute("aria-hidden", "true");
    const c = effectActivationGlowColor(typeLower, success);
    el.style.left = `${rect.left - pad}px`;
    el.style.top = `${rect.top - pad}px`;
    el.style.width = `${rect.width + pad * 2}px`;
    el.style.height = `${rect.height + pad * 2}px`;
    document.body.appendChild(el);
    const anim = el.animate(
      [
        { boxShadow: `0 0 0 3px ${c}`, opacity: 0.98 },
        { boxShadow: `0 0 52px 28px ${c}`, opacity: 0 },
      ],
      { duration: 1100, easing: "ease-out", fill: "forwards" },
    );
    await anim.finished.catch(() => {});
    el.remove();
  }

  /** @param {string} pid "A" | "B" */
  function manaBarElForPlayerId(pid) {
    if (pid === "A") return document.getElementById("manaBarA");
    if (pid === "B") return document.getElementById("manaBarB");
    return null;
  }

  /** @param {string} pid "A" | "B" */
  function energizedBarElForPlayerId(pid) {
    if (pid === "A") return document.getElementById("energizedBarA");
    if (pid === "B") return document.getElementById("energizedBarB");
    return null;
  }

  function parseBarLabelValue(labelEl) {
    const txt = String(labelEl?.textContent || "");
    const m = txt.match(/^\s*(\d+)\s*\/\s*(\d+)\s*$/);
    if (!m) return null;
    return { cur: Number(m[1]), max: Number(m[2]) };
  }

  function manaBarPartsForPlayer(pid) {
    if (pid === "A") return { manaFill: manaFillA, manaLabel: manaLabelA, enFill: energizedFillA, enLabel: energizedLabelA };
    if (pid === "B") return { manaFill: manaFillB, manaLabel: manaLabelB, enFill: energizedFillB, enLabel: energizedLabelB };
    return null;
  }

  /**
   * Applies the mana burn delta to HUD bars immediately in the activation step so values
   * update alongside the card glow (instead of waiting for the chain-end snapshot paint).
   * @param {string} affectedPid
   * @param {number} amount
   */
  function applyImmediateManaBurnHudDelta(affectedPid, amount) {
    const burn = Math.max(0, Number(amount) || 0);
    if (!burn) return;
    const parts = manaBarPartsForPlayer(affectedPid);
    if (!parts) return;
    const manaNow = parseBarLabelValue(parts.manaLabel);
    const enNow = parseBarLabelValue(parts.enLabel);
    if (!manaNow || !enNow) return;
    let rem = burn;
    const manaAfter = Math.max(0, manaNow.cur - rem);
    rem -= Math.max(0, manaNow.cur - manaAfter);
    const enAfter = Math.max(0, enNow.cur - rem);
    setBar(parts.manaFill, parts.manaLabel, manaAfter, manaNow.max);
    setBar(parts.enFill, parts.enLabel, enAfter, enNow.max);
  }

  /**
   * Short blue pulse on the mana bar when Energy Gain's effect activation succeeds (+4 mana).
   * Runs after activate_card (post-ignition burn), not when the card enters the ignition slot.
   * Awaited so the next queued activate_card / reaction FX waits until this finishes.
   * @param {HTMLElement | null} manaBarEl
   * @returns {Promise<void>}
   */
  async function playManaBarEnergyGainGlow(manaBarEl) {
    if (!manaBarEl) return;
    manaBarEl.classList.add("pm-mana-gain-flash");
    await new Promise((resolve) => {
      const done = () => {
        manaBarEl.removeEventListener("animationend", done);
        manaBarEl.classList.remove("pm-mana-gain-flash");
        resolve(undefined);
      };
      manaBarEl.addEventListener("animationend", done, { once: true });
      window.setTimeout(done, 900);
    });
  }

  /**
   * Red pulse on both mana bars (regular + energized) of the affected player when Mana Burn resolves.
   * Both animations run in parallel; the promise resolves when both finish.
   * @param {string} affectedPid "A" | "B" — the player whose mana is being burned
   * @returns {Promise<void>}
   */
  async function playManaBarManaBurnGlow(affectedPid) {
    const flash = (el) => {
      if (!el) return Promise.resolve();
      el.classList.add("pm-mana-burn-flash");
      return new Promise((resolve) => {
        const done = () => {
          el.removeEventListener("animationend", done);
          el.classList.remove("pm-mana-burn-flash");
          resolve(undefined);
        };
        el.addEventListener("animationend", done, { once: true });
        window.setTimeout(done, 900);
      });
    };
    await Promise.all([flash(manaBarElForPlayerId(affectedPid)), flash(energizedBarElForPlayerId(affectedPid))]);
  }

  /**
   * Buffers a server `activate_card` until the matching snapshot apply runs (so banner can precede FX).
   * @param {object} payload
   */
  function bufferServerActivationFx(payload) {
    pendingActivateCardPayloads.push(payload);
  }

  /**
   * Drains buffered `activate_card` payloads into the activation FX worker queue.
   * @param {object | null} layoutSnap snapshot to use for DOM/wrap lookup (pre-resolve); omit for non-reaction FX.
   * @param {object | null} [cooldownDeferral] from runTurnStartResourceSequence when ignition resolved with deferred cooldown odometers.
   * @returns {boolean} true if deferral was attached to a queued payload (activate_card will run cooldown odometers).
   */
  function flushPendingActivationFx(layoutSnap, cooldownDeferral) {
    if (pendingActivateCardPayloads.length === 0) {
      return false;
    }
    const batch = pendingActivateCardPayloads;
    pendingActivateCardPayloads = [];
    let rem = cooldownDeferral?.deferCooldownAfterActivation ? cooldownDeferral : null;
    let attached = false;
    for (const pl of batch) {
      const wrapped = layoutSnap ? { ...pl, _layoutSnap: layoutSnap } : { ...pl };
      if (
        rem &&
        String(pl.playerId || "") === rem.turnStarter &&
        String(pl.cardId || "") === rem.arrivingIgnitionCardId
      ) {
        wrapped._cooldownOdometerDeferral = rem;
        attached = true;
        rem = null;
      }
      enqueueServerActivationFx(wrapped);
    }
    return attached;
  }

  /**
   * Awaits the activation FX worker if a glow/fly sequence is in progress.
   * @returns {Promise<void>}
   */
  async function waitActivationFxWorkerIdle() {
    if (activationFxWorkerPromise) {
      try {
        await activationFxWorkerPromise;
      } catch (_) {
        /* worker clears queue on error */
      }
    }
  }

  /**
   * Queues server `activate_card` frames so glow/fly run one after another until the pile is drained.
   * @param {object} payload
   */
  function enqueueServerActivationFx(payload) {
    activationFxQueue.push(payload);
    if (activationFxWorkerPromise) {
      return;
    }
    activationFxWorkerPromise = (async () => {
      try {
        while (activationFxQueue.length > 0) {
          const pl = activationFxQueue.shift();
          const p = String(pl.playerId || "");
          const c = String(pl.cardId || "");
          if (!p || !c) continue;
          await runEffectActivationClientSequence(pl);
        }
      } finally {
        activationFxWorkerPromise = null;
      }
    })();
  }

  /**
   * Runs activation glow for server `activate_card` (effect resolution after reaction pile / ignition).
   * Glow (and optional mana pulse) runs while the card still occupies the ignition slot; only after those
   * finish is the slot cleared, the card shown in cooldown, and other recharge odometers applied.
   * @param {object} payload
   */
  async function runEffectActivationClientSequence(payload) {
    send("client_fx_hold", {});
    try {
      const pid = String(payload.playerId || "");
      const cid = String(payload.cardId || "");
      if (ignitionBlueHold && ignitionBlueHold.owner === pid && ignitionBlueHold.cardId === cid) {
        ignitionBlueHold = null;
      }
      const localPID = playerEl.value;
      const isSelf = pid === localPID;
      const layoutSnap = payload._layoutSnap || lastSnapshot;
      let wrap = reactionStackCardWrap(layoutSnap, localPID, cid, pid);
      if (!wrap) {
        wrap = isSelf ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
      }
      const typeLower = String(payload.cardType || getCardDef(cid)?.type || "").toLowerCase();
      const success = !!payload.success;
      const retainIgnition = !!payload.retainIgnition;
      const negatesActivationOf = String(payload.negatesActivationOf || "");
      const cooldownDeferral = payload._cooldownOdometerDeferral;
      const cooldownContainer = isSelf ? pmEl.cooldownCardsSelf : pmEl.cooldownCardsOpp;
      const pidSnap = lastSnapshot?.players?.find((p) => p.playerId === pid);
      const failExtraMs = !success ? 3200 : 0;
      blockGameplayInputForEffects(2000 + failExtraMs);
      const glowPromise = playEffectActivationGlow(wrap, typeLower, success);
      const oppPid = pid === "A" ? "B" : "A";
      const manaBarPromise =
        success && cid === "energy-gain"
          ? playManaBarEnergyGainGlow(manaBarElForPlayerId(pid))
          : success && cid === "mana-burn"
            ? playManaBarManaBurnGlow(oppPid)
            : Promise.resolve();
      if (success && cid === "mana-burn") {
        const oppHud = hudForSeat(layoutSnap, oppPid) || hudForSeat(lastSnapshot, oppPid);
        const burnAmount = Number(getCardDef(String(oppHud?.ignitionCard || ""))?.mana || 0);
        applyImmediateManaBurnHudDelta(oppPid, burnAmount);
      }
      await Promise.all([glowPromise, manaBarPromise]);
      // If this activation negated another player's card activation, place the negate overlay
      // now — before the next activate_card event (the negated card's fail animation) runs.
      if (negatesActivationOf) {
        applyNegateOverlayToIgnition(negatesActivationOf);
      }
      if (!success) {
        const vis = ignitionActivationCardVisual(wrap);
        await playNegationChromaSequence(vis);
      }
      if (retainIgnition) {
        if (lastSnapshot) renderIgnitionZone(lastSnapshot);
      } else {
        const clearedIgnitionCardEl = isSelf ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
        const clearedIgnitionCounterEl = isSelf ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;
        if (clearedIgnitionCardEl) {
          clearedIgnitionCardEl.innerHTML = "";
          delete clearedIgnitionCardEl.dataset.cardId;
          clearedIgnitionCardEl.classList.remove("pm-ignition-staged");
          clearedIgnitionCardEl.querySelectorAll(".pm-ignition-staged-overlay").forEach((el) => el.remove());
        }
        if (clearedIgnitionCounterEl) {
          clearedIgnitionCounterEl.classList.add("hidden");
        }
        if (cooldownDeferral) {
          await applyIgnitionResolveThenCooldownOdometers(cooldownDeferral);
          if (lastSnapshot) renderIgnitionZone(lastSnapshot);
        } else if (cooldownContainer && pidSnap) {
          renderCooldownList(cooldownContainer, pidSnap.cooldownPreview || []);
        }
      }
    } finally {
      send("client_fx_release", {});
    }
  }

  /**
   * Full-width "Resolving Effects" banner: width immediate, height grows from center in 0.5s; total ~1.5s.
   * @returns {Promise<void>}
   */
  function showResolvingEffectsBanner() {
    return new Promise((resolve) => {
      const mount = document.body;
      let el = document.getElementById("pmResolvingEffectsBanner");
      if (!el) {
        el = document.createElement("div");
        el.id = "pmResolvingEffectsBanner";
        el.className = "pm-resolving-effects-banner";
        el.setAttribute("role", "status");
        mount.appendChild(el);
      }
      el.textContent = t("resolvingEffects");
      el.classList.remove("pm-resolving-effects-banner--in", "pm-resolving-effects-banner--out");
      el.style.animation = "none";
      void el.offsetWidth;
      el.style.animation = "";
      void el.offsetWidth;
      el.classList.add("pm-resolving-effects-banner--in");
      setTimeout(() => {
        // Swap out --in for --out in one paint so we never fall back to the base (hidden) rule.
        requestAnimationFrame(() => {
          el.classList.remove("pm-resolving-effects-banner--in");
          void el.offsetWidth;
          el.classList.add("pm-resolving-effects-banner--out");
        });
        setTimeout(() => {
          el.classList.remove("pm-resolving-effects-banner--out");
          resolve();
        }, 400);
      }, 1500);
    });
  }

  /**
   * @param {object | null} prevSnap
   * @param {object} nextSnap
   * @returns {boolean}
   */
  function shouldRunReactionStackResolve(prevSnap, nextSnap) {
    if (!prevSnap || !nextSnap) return false;
    const p = prevSnap.reactionWindow || {};
    const n = nextSnap.reactionWindow || {};
    const trig = String(p.trigger || "");
    if (trig !== "" && trig !== "ignite_reaction" && trig !== "capture_attempt") {
      return false;
    }
    // Authoritative signal is stackSize (queued cards), not open: snapshots may omit or vary open.
    const hadStack = Number(p.stackSize || 0) > 0;
    const nextOpen = !!n.open;
    const nextSz = Number(n.stackSize || 0);
    const stackCleared = !nextOpen || nextSz === 0;
    return hadStack && stackCleared;
  }

  /**
   * Builds ordered resolve steps (LIFO) from the previous snapshot's reaction stack preview.
   * @param {object} prevSnap
   * @returns {{ cardId: string, owner: string }[]}
   */
  function reactionStackResolveOrderFromPrev(prevSnap) {
    const rw = prevSnap?.reactionWindow || {};
    const raw = Array.isArray(rw.stackCards) ? rw.stackCards : [];
    if (raw.length > 0) {
      return [...raw].reverse().map((e) => ({
        cardId: String(e.cardId || ""),
        owner: String(e.owner || ""),
      }));
    }
    const stagedId = String(rw.stagedCardId || "");
    const stagedOwner = String(rw.stagedOwner || "");
    const sz = Number(rw.stackSize || 0);
    if (!stagedId || !stagedOwner || sz <= 0) return [];
    // Without stackCards we only know the visible top; animating the same id N times is wrong.
    return [{ cardId: stagedId, owner: stagedOwner }];
  }

  /**
   * Number of cards queued on the reaction stack (uses stackSize when stackCards is omitted).
   * @param {object | null} prevSnap
   * @returns {number}
   */
  function reactionStackBannerCardCount(prevSnap) {
    const rw = prevSnap?.reactionWindow || {};
    const raw = Array.isArray(rw.stackCards) ? rw.stackCards : [];
    return Math.max(raw.length, Number(rw.stackSize || 0));
  }

  /**
   * "Resolving Effects" banner + glow flush for the reaction stack resolve sequence.
   * Banner shows only when 2+ effects are activating (reaction card + ignition card, or 2+ reactions).
   * Per-card glow runs after the banner.
   * @param {object} prevSnap
   * @param {object} nextSnap
   * @returns {Promise<void>}
   */
  async function runReactionStackResolveSequence(prevSnap, _nextSnap) {
    const bannerCount = reactionStackBannerCardCount(prevSnap);
    const pendingCount = pendingActivateCardPayloads.length;
    const steps = reactionStackResolveOrderFromPrev(prevSnap);
    if (steps.length === 0 && pendingCount === 0) {
      return;
    }
    renderPlaymat(prevSnap);
    await new Promise((r) => requestAnimationFrame(() => r()));
    // Drain any in-progress activation FX from a previous snapshot-apply step before showing the
    // banner. Without this, an orphaned FX worker (started before this apply but left running due to
    // an error swallowed by the snapshotApplyChain catch) could play card glows while the banner is
    // visible, since the banner only covers the center strip and ignition zones remain visible.
    await waitActivationFxWorkerIdle();
    // Show banner only when 2+ effects are activating simultaneously (e.g. retribution + power card).
    if (Math.max(bannerCount, pendingCount) >= 2) {
      await showResolvingEffectsBanner();
    }
    flushPendingActivationFx(prevSnap);
  }

  /**
   * Renders the top queued reaction on the correct side of the playmat for the current viewer.
   */
  function renderReactionStagedIfAny(snapshot, localPID) {
    const rw = snapshot?.reactionWindow;
    const sid = rw?.stagedCardId;
    const sor = rw?.stagedOwner;
    const stackCards = Array.isArray(rw?.stackCards) ? rw.stackCards : [];
    const clearStagedDecor = () => {
      [pmEl.ignitionCardSelf, pmEl.ignitionCardOpp].forEach((slot) => {
        if (!slot) return;
        slot.classList.remove("pm-ignition-staged");
        slot.querySelectorAll(".pm-ignition-staged-overlay").forEach((el) => el.remove());
      });
    };
    const mountCardIntoSlot = (slot, counterEl, cardId) => {
      if (!slot || !cardId) return;
      const def = getCardDef(cardId);
      slot.innerHTML = "";
      slot.dataset.cardId = String(cardId);
      slot.classList.remove("pm-ignition-staged");
      slot.querySelectorAll(".pm-ignition-staged-overlay").forEach((el) => el.remove());
      if (counterEl) counterEl.classList.add("hidden");
      if (!def) return;
      const cid = String(cardId);
      const card = createPowerCard({
        type: def.type,
        name: def.name,
        description: def.description,
        example: def.example,
        mana: def.mana,
        ignition: def.ignition,
        cooldown: def.cooldown,
        cardWidth: "220px",
        showExampleInitially: matchHandExampleMode.get(cid) === true,
        onExampleToggle: (showing) => {
          matchHandExampleMode.set(cid, showing);
          syncAllMatchPowerCardMinis(cid, showing);
        },
      });
      slot.appendChild(card);
      attachCardHover(slot, { ...def, id: cid, manaCost: def.mana });
    };
    // Capture-attempt stacks can have 2+ Counters with no base ignition cards.
    // Keep the latest card from each owner visible in their ignition slots so
    // the next card does not "disappear" while the previous one animates.
    if (rw?.trigger === "capture_attempt" && stackCards.length > 1) {
      const byOwner = new Map();
      for (const entry of stackCards) {
        const owner = String(entry?.owner || "");
        const cardId = String(entry?.cardId || "");
        if (owner === "" || cardId === "") continue;
        byOwner.set(owner, cardId);
      }
      const selfCardId = byOwner.get(localPID) || "";
      const oppPID = localPID === "A" ? "B" : "A";
      const oppCardId = byOwner.get(oppPID) || "";
      if (selfCardId) {
        mountCardIntoSlot(pmEl.ignitionCardSelf, pmEl.ignitionCounterSelf, selfCardId);
      }
      if (oppCardId) {
        mountCardIntoSlot(pmEl.ignitionCardOpp, pmEl.ignitionCounterOpp, oppCardId);
      }
    }
    if (!sid || !sor) {
      clearStagedDecor();
      return;
    }
    const slot = pickReactionStagedSlot(snapshot, localPID, sor);
    if (!slot) {
      clearStagedDecor();
      return;
    }
    clearStagedDecor();
    const baseId = slot.dataset.cardId ? String(slot.dataset.cardId) : "";
    const overlayNeeded = baseId !== "" && baseId !== String(sid);
    const counterEl = slot === pmEl.ignitionCardSelf ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;

    const mountStagedCard = (parent) => {
      const def = getCardDef(sid);
      if (!def) return;
      const cid = String(sid);
      const card = createPowerCard({
        type: def.type,
        name: def.name,
        description: def.description,
        example: def.example,
        mana: def.mana,
        ignition: def.ignition,
        cooldown: def.cooldown,
        cardWidth: "220px",
        showExampleInitially: matchHandExampleMode.get(cid) === true,
        onExampleToggle: (showing) => {
          matchHandExampleMode.set(cid, showing);
          syncAllMatchPowerCardMinis(cid, showing);
        },
      });
      parent.appendChild(card);
      attachCardHover(parent, { ...def, id: cid, manaCost: def.mana });
    };

    if (overlayNeeded) {
      slot.classList.add("pm-ignition-staged");
      const layer = document.createElement("div");
      layer.className = "pm-ignition-staged-overlay";
      slot.appendChild(layer);
      mountStagedCard(layer);
      return;
    }

    slot.classList.add("pm-ignition-staged");
    slot.innerHTML = "";
    slot.dataset.cardId = String(sid);
    if (counterEl) counterEl.classList.add("hidden");
    mountStagedCard(slot);
  }

  /**
   * Immediately adds the negate overlay to the ignition card of the given player without
   * waiting for a snapshot. Called right after the negating card's glow animation completes
   * so the overlay is visible before the negated card's own fail animation plays.
   * @param {string} targetPid - player ID whose ignition card was negated
   */
  function applyNegateOverlayToIgnition(targetPid) {
    const localPID = playerEl.value;
    const isSelf = targetPid === localPID;
    const cardEl = isSelf ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
    if (!cardEl || !cardEl.dataset.cardId) return;
    // Remove any stale overlay before inserting a fresh one.
    cardEl.querySelectorAll(".pm-ignition-negate-overlay").forEach((el) => el.remove());
    const ov = document.createElement("div");
    ov.className = "pm-ignition-negate-overlay";
    ov.setAttribute("aria-hidden", "true");
    const img = document.createElement("img");
    img.className = "pm-ignition-negate-overlay__img";
    img.src = "/public/negate.png";
    img.alt = "";
    ov.appendChild(img);
    cardEl.appendChild(ov);
  }

  function renderIgnitionZone(snapshot) {
    const localPID = playerEl.value;
    const selfHud = hudForSeat(snapshot, localPID);
    const oppHud = snapshot?.players?.find((p) => p.playerId !== localPID);

    if (pmEl.ignitionSelf) {
      pmEl.ignitionSelf.classList.remove("pm-ignition-global-blocked");
      pmEl.ignitionSelf.title = "";
    }
    if (pmEl.ignitionOpp) {
      pmEl.ignitionOpp.classList.remove("pm-ignition-global-blocked");
      pmEl.ignitionOpp.title = "";
    }

    /**
     * @param {object|undefined} hud
     * @param {boolean} isSelf
     */
    const fillOne = (hud, isSelf) => {
      const cardEl = isSelf ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
      const counterEl = isSelf ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;
      const occupied = !!hud?.ignitionOn;
      if (!occupied) {
        if (cardEl) {
          cardEl.innerHTML = "";
          delete cardEl.dataset.cardId;
          cardEl.classList.remove("pm-ignition-staged");
          cardEl.querySelectorAll(".pm-ignition-staged-overlay").forEach((el) => el.remove());
        }
        if (counterEl) counterEl.classList.add("hidden");
        return;
      }
      const cardId = hud.ignitionCard;
      const turns = hud.ignitionTurnsRemaining ?? 0;
      if (!cardEl) return;
      cardEl.innerHTML = "";
      cardEl.dataset.cardId = String(cardId || "");
      cardEl.classList.remove("pm-ignition-staged");
      cardEl.querySelectorAll(".pm-ignition-staged-overlay").forEach((el) => el.remove());
      if (counterEl) {
        counterEl.textContent = `${turns}t`;
        counterEl.classList.remove("hidden");
      }
      const def = getCardDef(cardId);
      if (def) {
        const cid = String(cardId);
        const card = createPowerCard({
          type: def.type,
          name: def.name,
          description: def.description,
          example: def.example,
          mana: def.mana,
          ignition: def.ignition,
          cooldown: def.cooldown,
          cardWidth: "220px",
          showExampleInitially: matchHandExampleMode.get(cid) === true,
          onExampleToggle: (showing) => {
            matchHandExampleMode.set(cid, showing);
            syncAllMatchPowerCardMinis(cid, showing);
          },
        });
        cardEl.appendChild(card);
        attachCardHover(cardEl, { ...def, id: cid, manaCost: def.mana });
        if (hud.ignitionEffectNegated) {
          const ov = document.createElement("div");
          ov.className = "pm-ignition-negate-overlay";
          ov.setAttribute("aria-hidden", "true");
          const img = document.createElement("img");
          img.className = "pm-ignition-negate-overlay__img";
          img.src = "/public/negate.png";
          img.alt = "";
          ov.appendChild(img);
          cardEl.appendChild(ov);
        }
      }
    };

    fillOne(selfHud, true);
    fillOne(oppHud, false);
    renderReactionStagedIfAny(snapshot, localPID);
    if (pmEl.ignitionSelf) {
      pmEl.ignitionSelf.classList.toggle("pm-ignition-targeting", !!igniteTargetFlow);
    }
  }

  function renderCooldownZone(self, opp) {
    renderCooldownList(pmEl.cooldownCardsSelf, self.cooldownPreview || [], { isOpponentZone: false, self, opp });
    renderCooldownList(pmEl.cooldownCardsOpp, opp.cooldownPreview || [], { isOpponentZone: true, self, opp });
  }

  function hasValidRetaliateTarget(snapshot, pid) {
    const opp = hudForSeat(snapshot, oppositeSeat(pid));
    if (!opp) return false;
    return (opp.cooldownPreview || []).some((entry) => canRetaliateTargetCooldownEntry(entry, opp));
  }

  function canRetaliateTargetCooldownEntry(entry, oppHud) {
    const def = getCardDef(entry?.cardId);
    if (String(def?.type || "").toLowerCase() !== "power") return false;
    const cost = Number(entry?.manaCost ?? def?.mana ?? 0);
    return Number(oppHud?.mana || 0) >= cost;
  }

  /**
   * Renders cooldown as a Balatro-style row: left-aligned, soonest-to-resolve left;
   * overlap tightens so the row fits the container width.
   * @param {HTMLElement | null} container
   * @param {Array<{ cardId: string, turnsRemaining: number }>} allEntries
   */
  function renderCooldownList(container, allEntries, opts = {}) {
    if (!container) return;
    container.innerHTML = "";
    const entries = sortCooldownEntriesForDisplay(allEntries || []);
    const stackTotal = Math.max(1, entries.length);
    container.style.setProperty("--cd-stack-n", String(stackTotal));

    const stack = document.createElement("div");
    stack.className = "pm-cooldown-stack";
    container.appendChild(stack);

    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i];
      const def = getCardDef(entry.cardId);
      const wrap = document.createElement("div");
      wrap.className = "pm-cooldown-card-wrap";
      wrap.dataset.cooldownIndex = String(i);
      wrap.dataset.cardId = entry.cardId;
      wrap.style.setProperty("--i", String(i));
      const canRetaliateTarget =
        retaliateTargetFlow !== null &&
        opts.isOpponentZone === true &&
        canRetaliateTargetCooldownEntry(entry, opts.opp);
      wrap.classList.toggle("pm-cooldown-card-wrap--retaliate-target", canRetaliateTarget);

      if (def) {
        const cid = String(entry.cardId);
        const card = createPowerCard({
          type: def.type,
          name: def.name,
          description: def.description,
          example: def.example,
          mana: def.mana,
          ignition: def.ignition,
          cooldown: def.cooldown,
          cardWidth: "220px",
          showExampleInitially: matchHandExampleMode.get(cid) === true,
          onExampleToggle: (showing) => {
            matchHandExampleMode.set(cid, showing);
            syncAllMatchPowerCardMinis(cid, showing);
          },
        });
        wrap.appendChild(card);
        attachCardHover(wrap, { ...def, id: cid, manaCost: def.mana });
      } else {
        const fb = document.createElement("div");
        fb.className = "pm-sleeve-card";
        wrap.appendChild(fb);
      }

      const turnsEl = document.createElement("span");
      turnsEl.className = "pm-cooldown-turns";
      turnsEl.textContent = `${entry.turnsRemaining}t`;
      wrap.appendChild(turnsEl);
      if (canRetaliateTarget) {
        wrap.addEventListener("click", (ev) => {
          if (ev.target instanceof Element && ev.target.closest(".power-card__toggle")) return;
          ev.preventDefault();
          ev.stopPropagation();
          const handIndex = retaliateTargetFlow?.handIndex;
          if (handIndex == null) return;
          retaliateTargetFlow = null;
          send("queue_reaction", { handIndex, targetCardId: entry.cardId });
          if (lastSnapshot) renderPlaymat(lastSnapshot);
        });
      }
      stack.appendChild(wrap);
    }
  }

  function renderHandZone(self, opp) {
    renderOwnHand(pmEl.handSelf, self);
    renderOppHand(pmEl.handOpp, opp);
  }

  function actionPromptForSnapshot(snapshot) {
    if (disruptionBanishFlow !== null) return t("disruptionBanishHint");
    if (retaliateTargetFlow !== null) return t("retaliateTargetHint");
    if (deckSearchOpenForCard) return t("selectDeckSearchCardHint");
    const zipLinePending = viewerZipLinePendingSource(snapshot);
    if (zipLinePending) return t("selectZipLineDestinationHint");
    const pendingBoardTarget = viewerPendingBoardTargetEffect(snapshot);
    if (pendingBoardTarget) {
      if (pendingBoardTarget.cardId === "mind-control") return t("selectOpponentPieceHint");
      if (pendingBoardTarget.cardId === "sacrifice-of-the-masses") return t("selectOwnPawnHint");
      return t("selectOwnPieceHint");
    }
    if (igniteTargetFlow?.stage === "picking") {
      if (igniteTargetFlow.cardId === "piece-swap") {
        return igniteTargetFlow.firstPick ? t("selectPieceSwapSecondHint") : t("selectPieceSwapFirstHint");
      }
      if (igniteTargetFlow.cardId === "mind-control") return t("selectOpponentPieceHint");
      if (igniteTargetFlow.cardId === "sacrifice-of-the-masses") return t("selectOwnPawnHint");
      return t("selectOwnPieceHint");
    }
    if (igniteTargetFlow?.stage === "placed") return t("selectIgnitionTargetHint");
    if (canPassReactionPriority(snapshot, playerEl.value)) return t("confirmPlayHint");
    return "";
  }

  function renderPlayerActionBanner(snapshot) {
    const banner = pmEl.playerActionBanner;
    const hint = pmEl.playerActionHint;
    if (!banner || !hint) return;
    const canPass = canPassReactionPriority(snapshot, playerEl.value);
    const prompt = actionPromptForSnapshot(snapshot);
    hint.textContent = prompt;
    banner.classList.toggle("hidden", !prompt && !canPass);
    banner.classList.toggle("pm-action-banner--confirm", canPass);
    if (reactionPassBtnEl) {
      reactionPassBtnEl.textContent = t("reactionPassOk");
      reactionPassBtnEl.classList.toggle("hidden", !canPass);
      reactionPassBtnEl.disabled = !canPass || !isGameplayInputOpen();
    }
  }

  /**
   * Returns the inner node where hand cards are mounted (preserves the Hand label).
   * @param {HTMLElement} container
   * @returns {HTMLElement | null}
   */
  function getHandCardsRoot(container) {
    if (!container) return null;
    let root = container.querySelector(".pm-hand-cards");
    if (!root) {
      root = document.createElement("div");
      root.className = "pm-hand-cards";
      container.appendChild(root);
    }
    return root;
  }

  function renderOwnHand(container, self) {
    if (!container) return;
    const cardsRoot = getHandCardsRoot(container);
    if (!cardsRoot) return;
    cardsRoot.innerHTML = "";
    const hand = self.hand || [];
    for (const idx of [...mulliganPick]) {
      if (idx < 0 || idx >= hand.length) mulliganPick.delete(idx);
    }
    const snap = lastSnapshot;
    const mr = snap?.mulliganReturned || {};
    const myPid = playerEl.value;
    const myMulliganDone = mr[myPid] !== undefined && mr[myPid] >= 0;
    const mulliganChoose = !!(snap?.mulliganPhaseActive && !myMulliganDone);
    if (hand.length === 0) {
      cardsRoot.classList.remove("pm-hand-cards--overlap");
      return;
    }

    const n = Math.max(1, hand.length);
    cardsRoot.classList.add("pm-hand-cards--overlap");
    cardsRoot.style.setProperty("--cd-stack-n", String(n));

    const stack = document.createElement("div");
    stack.className = "pm-hand-stack";
    stack.classList.toggle("pm-hand-stack--mulligan", mulliganChoose);
    cardsRoot.appendChild(stack);

    for (let i = 0; i < hand.length; i++) {
      const entry = hand[i];
      const wrap = document.createElement("div");
      wrap.className = "pm-hand-card-wrap";
      wrap.dataset.handIndex = String(i);
      wrap.dataset.cardId = entry.cardId;

      const def = getCardDef(entry.cardId);
      if (def) {
        const cid = entry.cardId;
        const card = createPowerCard({
          type: def.type,
          name: def.name,
          description: def.description,
          example: def.example,
          mana: def.mana,
          ignition: def.ignition,
          cooldown: def.cooldown,
          cardWidth: "220px",
          showExampleInitially: matchHandExampleMode.get(cid) === true,
          onExampleToggle: (showing) => {
            matchHandExampleMode.set(cid, showing);
            syncAllMatchPowerCardMinis(cid, showing);
          },
        });
        wrap.appendChild(card);
      }
      wrap.style.setProperty("--i", String(i));
      const canActivate = !mulliganChoose && canActivateHandCard(snap, myPid, entry);
      wrap.classList.toggle("pm-hand-card-wrap--inactive", !canActivate);
      wrap.classList.toggle("pm-hand-card-wrap--mulligan-return", mulliganChoose && mulliganPick.has(i));
      // Banish selection mode visual: highlight Power cards as targets, mark the pending Disruption card.
      const inBanishMode = disruptionBanishFlow !== null && !mulliganChoose;
      wrap.classList.toggle("pm-hand-card-wrap--banish-target", inBanishMode && canActivate);
      wrap.classList.toggle(
        "pm-hand-card-wrap--disruption-pending",
        inBanishMode && i === disruptionBanishFlow?.handIndex,
      );
      if (mulliganChoose) {
        wrap.addEventListener("click", (ev) => {
          if (ev.target instanceof Element && ev.target.closest(".power-card__toggle")) return;
          ev.preventDefault();
          ev.stopPropagation();
          if (mulliganPick.has(i)) mulliganPick.delete(i);
          else mulliganPick.add(i);
          if (lastSnapshot) renderPlaymat(lastSnapshot);
        });
      } else if (inBanishMode && canActivate) {
        // During disruption banish selection, a click on any active (Power) card completes the banish.
        wrap.addEventListener("click", (ev) => {
          if (ev.target instanceof Element && ev.target.closest(".power-card__toggle")) return;
          ev.preventDefault();
          ev.stopPropagation();
          const snapAtClick = lastSnapshot;
          sendHandCardAction(snapAtClick, i);
          // Re-render to clear banish mode visuals while waiting for server snapshot.
          if (lastSnapshot) renderPlaymat(lastSnapshot);
        });
      }
      stack.appendChild(wrap);
    }

    wireOverlapStackPreviewHover(stack, hand, {
      wrapSelector: ".pm-hand-card-wrap",
      peekClassName: "pm-hand-card-wrap--peek",
      hiddenSelector: null,
    });
    if (!mulliganChoose) {
      wireHandStackPointerDrag(stack);
    }
  }

  function isReactionFirstResponseTurn(snapshot, pid) {
    const rw = snapshot?.reactionWindow || {};
    if (!rw.open) return false;
    if (!rw.actor || rw.actor === pid) return false;
    const stackSize = Number(rw.stackSize || 0);
    return Number.isFinite(stackSize) && stackSize === 0;
  }

  function currentReactionResponder(snapshot) {
    const rw = snapshot?.reactionWindow || {};
    if (!rw.open || !rw.actor) return "";
    const stackSize = Number(rw.stackSize || 0);
    if (!Number.isFinite(stackSize) || stackSize <= 0) return oppositeSeat(rw.actor);
    if (Array.isArray(rw.stackCards) && rw.stackCards.length > 0) {
      const top = rw.stackCards[rw.stackCards.length - 1];
      if (top?.owner) return oppositeSeat(top.owner);
    }
    if (rw.stagedOwner) return oppositeSeat(rw.stagedOwner);
    return oppositeSeat(rw.actor);
  }

  function oppositeSeat(pid) {
    return pid === "A" ? "B" : pid === "B" ? "A" : "";
  }

  function isReactionWindowOpen(snapshot) {
    return !!snapshot?.reactionWindow?.open;
  }

  /**
   * Attacker may pass the optional Blockade step after the defender queues Counterattack (capture_attempt).
   */
  function canAttackerPassAfterCounterQueued(snapshot, pid) {
    const rw = snapshot?.reactionWindow || {};
    if (!rw.open || rw.trigger !== "capture_attempt") return false;
    if (!rw.actor || rw.actor !== pid) return false;
    const stackSize = Number(rw.stackSize || 0);
    if (stackSize !== 1) return false;
    if ((rw.stagedCardId || "") !== "counterattack") return false;
    if ((rw.stagedOwner || "") === pid) return false;
    return true;
  }

  function canPassReactionPriority(snapshot, pid) {
    if (!snapshot || !pid) return false;
    const responder = currentReactionResponder(snapshot);
    if (!responder) return false;
    return responder === pid;
  }

  /**
   * True when this seat's ignition zone is occupied. Server rejects new normal ignite_card
   * for that seat until it clears.
   * @param {object|null} snapshot
   * @param {string} pid
   * @returns {boolean}
   */
  function isOwnIgnitionOccupied(snapshot, pid) {
    return !!hudForSeat(snapshot, pid)?.ignitionOn;
  }

  /**
   * Returns whether the hand card's type matches reactionWindow.eligibleTypes (case-insensitive).
   * Server metadata uses title case ("Counter"); card-metadata may use lowercase ("counter").
   */
  function cardMatchesReactionEligibleTypes(snapshot, handEntry) {
    const eligible = snapshot?.reactionWindow?.eligibleTypes;
    if (!Array.isArray(eligible) || eligible.length === 0) return true;
    const def = getCardDef(handEntry?.cardId);
    if (!def?.type) return false;
    const t = String(def.type).toLowerCase();
    return eligible.some((e) => String(e).toLowerCase() === t);
  }

  function canActivateHandCard(snapshot, pid, handEntry) {
    if (!snapshot || !pid || !handEntry) return false;
    // During disruption banish selection, only Power cards are valid banish targets.
    if (disruptionBanishFlow !== null) {
      const cardType = String(getCardDef(handEntry?.cardId)?.type || "").toLowerCase();
      return cardType === "power";
    }
    if (retaliateTargetFlow !== null) return false;
    if (isOwnIgnitionOccupied(snapshot, pid)) {
      return false;
    }
    if (isReactionWindowOpen(snapshot)) {
      if (!canPassReactionPriority(snapshot, pid)) return false;
      if (String(handEntry?.cardId || "") === "retaliate" && !hasValidRetaliateTarget(snapshot, pid)) return false;
      return cardMatchesReactionEligibleTypes(snapshot, handEntry);
    }
    if (isReactionFirstResponseTurn(snapshot, pid)) {
      return true;
    }
    const isMyTurn = snapshot.turnPlayer === pid;
    if (!isMyTurn) return false;
    // Piece Swap requires at least one valid swap pair to exist. If not, the card would go to
    // ignition but leave the player stuck with nothing to select.
    if (String(handEntry?.cardId) === "piece-swap") {
      const board = snapshot?.board || [];
      const ownColor = pid === "A" ? "w" : "b";
      if (pieceSwapEligibleOwnSet(board, ownColor).size === 0) return false;
    }
    if (String(handEntry?.cardId) === "zip-line") {
      const board = snapshot?.board || [];
      if (!zipLineHasRoughPlayableTarget(board, pid)) return false;
    }
    if (String(handEntry?.cardId) === "sacrifice-of-the-masses") {
      const self = hudForSeat(snapshot, pid);
      if ((self?.hand || []).length >= 5) return false;
    }
    return true;
  }

  function cardRequiresTargetPieces(cardId) {
    return (Number(getCardDef(cardId)?.targets) || 0) > 0;
  }

  function sendHandCardAction(snapshot, handIndex) {
    if (!isGameplayInputOpen()) return;

    // Step 2 of disruption banish flow: user selected a Power card to banish.
    if (disruptionBanishFlow !== null) {
      const self = hudForSeat(snapshot, playerEl.value);
      const hand = self?.hand || [];
      const entry = hand[handIndex];
      const cardType = String(getCardDef(entry?.cardId)?.type || "").toLowerCase();
      if (cardType === "power") {
        const disruptionIdx = disruptionBanishFlow.handIndex;
        disruptionBanishFlow = null;
        send("queue_reaction", { handIndex: disruptionIdx, banishHandIndex: handIndex });
      } else {
        // Clicked non-Power card: cancel banish selection.
        disruptionBanishFlow = null;
        if (lastSnapshot) renderPlaymat(lastSnapshot);
      }
      return;
    }

    if (isReactionWindowOpen(snapshot)) {
      const self = hudForSeat(snapshot, playerEl.value);
      const hand = self?.hand || [];
      const entry = hand[handIndex];
      const cardType = String(getCardDef(entry?.cardId)?.type || "").toLowerCase();
      const rw = snapshot?.reactionWindow || {};
      const isFirstIgniteSlot = rw.trigger === "ignite_reaction" && Number(rw.stackSize || 0) === 0;

      if (cardType === "disruption" && isFirstIgniteSlot) {
        // Step 1: enter banish selection mode — do not send yet.
        disruptionBanishFlow = { handIndex };
        return; // finishPointer handles DOM restore + re-render
      }
      if (String(entry?.cardId || "") === "retaliate") {
        if (!hasValidRetaliateTarget(snapshot, playerEl.value)) return;
        retaliateTargetFlow = { handIndex };
        return;
      }

      send("queue_reaction", { handIndex });
      return;
    }
    const self = hudForSeat(snapshot, playerEl.value);
    const hand = self?.hand || [];
    const entry = hand[handIndex];
    const cardId = String(entry?.cardId || "");
    if (cardRequiresTargetPieces(cardId)) {
      send("ignite_card", { handIndex });
      igniteTargetFlow = { stage: "placed", cardId };
      selectedFrom = null;
      highlightedMoves = [];
      if (snapshot?.board) renderBoard(snapshot.board);
      if (lastSnapshot) renderPlaymat(lastSnapshot);
      return;
    }
    send("ignite_card", { handIndex });
  }

  function updateReactionPassButton(snapshot) {
    renderPlayerActionBanner(snapshot);
  }

  function renderOppHand(container, opp) {
    if (!container) return;
    const cardsRoot = getHandCardsRoot(container);
    if (!cardsRoot) return;
    cardsRoot.innerHTML = "";
    const count = opp.handCount || 0;
    const sleeve = opp.sleeveColor || "blue";
    if (count === 0) {
      cardsRoot.classList.remove("pm-hand-cards--overlap");
      return;
    }

    const n = Math.max(1, count);
    cardsRoot.classList.add("pm-hand-cards--overlap");
    cardsRoot.style.setProperty("--cd-stack-n", String(n));

    const stack = document.createElement("div");
    stack.className = "pm-hand-stack";
    cardsRoot.appendChild(stack);

    for (let i = 0; i < count; i++) {
      const wrap = document.createElement("div");
      wrap.className = "pm-hand-card-wrap";
      wrap.style.setProperty("--i", String(i));

      const face = document.createElement("div");
      face.className = "pm-sleeve-card";
      face.style.backgroundImage = `url('${sleeveUrl(sleeve)}')`;
      wrap.appendChild(face);

      stack.appendChild(wrap);
    }
  }

  function updateDrawButton(snapshot, self) {
    const btn = pmEl.drawBtn;
    if (!btn) return;
    const isMyTurn = snapshot.turnPlayer === playerEl.value;
    const hasMana = (self.mana || 0) >= 2;
    const hasSpace = (self.handCount || 0) < 5;
    const gameOn = snapshot.gameStarted && !snapshot.matchEnded;
    const mulliganOn = !!snapshot.mulliganPhaseActive;
    btn.disabled = mulliganOn || !(isMyTurn && hasMana && hasSpace && gameOn) || !isGameplayInputOpen();
  }

  // ---------------------------------------------------------------------------
  // Playmat: card hover wiring
  // ---------------------------------------------------------------------------
  function attachCardHover(el, cardData) {
    el.addEventListener("mouseenter", () => showCardPreview(cardData, el));
    el.addEventListener("mouseleave", () => {
      if (pmPreviewCard === el) hideCardPreview();
    });
    el.addEventListener("mousemove", () => {
      if (pmPreviewCard === el) positionCardPreview(el);
    });
  }

  // Hand → ignition: shared state for HTML5 drop target + pointer drag.
  let draggingHandIndex = null;

  /**
   * First wrap (left-to-right DOM order) whose horizontal bounds contain clientX wins overlaps.
   * @param {number} clientX
   * @param {HTMLElement[]} wraps
   * @returns {number}
   */
  function overlapHitFirstWrapIndex(clientX, wraps) {
    for (let i = 0; i < wraps.length; i++) {
      const rr = wraps[i].getBoundingClientRect();
      if (clientX >= rr.left && clientX <= rr.right) return i;
    }
    return -1;
  }

  /**
   * Delegated hover for overlapping card rows (cooldown, banish, hand): wraps use
   * pointer-events none; stack resolves which card is under the cursor on X.
   * @param {HTMLElement} stack
   * @param {Array<{ cardId: string }>} entries
   * @param {{ wrapSelector: string, peekClassName: string, hiddenSelector: string | null }} opts
   */
  function wireOverlapStackPreviewHover(stack, entries, opts) {
    const { wrapSelector, peekClassName, hiddenSelector } = opts;
    let lastIdx = -1;

    function clearPeek() {
      stack.querySelectorAll(`.${peekClassName}`).forEach((el) => {
        el.classList.remove(peekClassName);
      });
    }

    function onMove(ev) {
      if (draggingHandIndex !== null) {
        if (lastIdx !== -1) hideCardPreview();
        lastIdx = -1;
        clearPeek();
        return;
      }
      const x = ev.clientX;
      const wraps = [...stack.querySelectorAll(wrapSelector)];
      let idx = overlapHitFirstWrapIndex(x, wraps);

      if (idx < 0 && hiddenSelector) {
        const hidden = stack.querySelector(hiddenSelector);
        if (hidden) {
          const hr = hidden.getBoundingClientRect();
          if (x >= hr.left && x <= hr.right) {
            if (lastIdx !== -1) hideCardPreview();
            lastIdx = -1;
            clearPeek();
            return;
          }
        }
      }

      if (idx < 0) {
        if (lastIdx !== -1) hideCardPreview();
        lastIdx = -1;
        clearPeek();
        return;
      }

      if (idx === lastIdx) {
        positionCardPreview(wraps[idx]);
        return;
      }
      lastIdx = idx;
      const entry = entries[idx];
      const def = entry ? getCardDef(entry.cardId) : null;
      clearPeek();
      wraps[idx].classList.add(peekClassName);
      if (!def) {
        hideCardPreview();
        return;
      }
      showCardPreview({ ...def, id: def.id != null ? def.id : entry.cardId, manaCost: def.mana }, wraps[idx]);
    }

    function onLeave() {
      lastIdx = -1;
      hideCardPreview();
      clearPeek();
    }

    stack.addEventListener("mousemove", onMove);
    stack.addEventListener("mouseleave", onLeave);
  }

  function clearDanglingHandDragCards() {
    document.querySelectorAll(".power-card--hand-dragging").forEach((card) => {
      if (!(card instanceof HTMLElement)) return;
      card.classList.remove("power-card--hand-dragging");
      card.style.position = "";
      card.style.left = "";
      card.style.top = "";
      card.style.width = "";
      card.style.zIndex = "";
      card.style.margin = "";
      card.style.pointerEvents = "";
      card.style.transform = "";
      card.style.transformOrigin = "";
      card.style.boxShadow = "";
      if (card.parentElement === document.body) {
        card.remove();
      }
    });
    draggingHandIndex = null;
    hideCardPreview();
  }

  /**
   * Pointer-based drag from hand stack to ignition (overlapping hand has pointer-events none on wraps).
   * Hides the large hover preview when a drag starts; lifts the real card (no ghost clone);
   * restores the card to its slot if not dropped on ignition.
   * @param {HTMLElement} stack
   */
  function wireHandStackPointerDrag(stack) {
    const dragThresholdPx = 10;
    /** @type {{ idx: number, entry: object, startX: number, startY: number, pointerId: number, armed: boolean, cardEl?: HTMLElement, wrapEl?: HTMLElement, grabOffsetX?: number, grabOffsetY?: number, dragScaleLifted?: number } | null} */
    let pending = null;

    /**
     * Clears fixed-position drag styling on a hand card and restores the slot placeholder.
     * @param {{ cardEl?: HTMLElement, wrapEl?: HTMLElement } | null} p
     */
    function resetHandDragVisual(p) {
      if (!p?.cardEl || !p.wrapEl) return;
      const card = p.cardEl;
      const wrap = p.wrapEl;
      card.classList.remove("power-card--hand-dragging");
      wrap.classList.remove("pm-hand-card-wrap--dragging");
      if (card.parentNode !== wrap) {
        wrap.appendChild(card);
      }
      card.style.position = "";
      card.style.left = "";
      card.style.top = "";
      card.style.width = "";
      card.style.zIndex = "";
      card.style.margin = "";
      card.style.pointerEvents = "";
      card.style.transform = "";
      card.style.transformOrigin = "";
      card.style.boxShadow = "";
      wrap.style.minHeight = "";
      stack.style.cursor = "";
    }

    function clearDragChrome() {
      stack.style.cursor = "";
      if (pmEl.ignitionSelf) {
        pmEl.ignitionSelf.classList.remove("pm-drop-active", "pm-drop-hover");
      }
      draggingHandIndex = null;
    }

    stack.addEventListener("pointerdown", (ev) => {
      if (ev.button !== 0) return;
      if (!isGameplayInputOpen()) return;
      clearDanglingHandDragCards();
      // Let the description/example toggle receive a normal click cycle; pointer capture
      // on the stack would steal pointerup/click from the button.
      const t = ev.target;
      if (t instanceof Element && t.closest(".power-card__toggle")) {
        return;
      }
      const snap = lastSnapshot;
      const localPID = playerEl.value;
      const self = snap?.players?.find((p) => p.playerId === localPID);
      const hand = self?.hand || [];
      if (hand.length === 0) return;

      const wraps = [...stack.querySelectorAll(".pm-hand-card-wrap")];
      const idx = overlapHitFirstWrapIndex(ev.clientX, wraps);
      if (idx < 0 || idx >= hand.length) return;

      const entry = hand[idx];
      const canActivate = canActivateHandCard(snap, localPID, entry);
      if (!canActivate) return;

      pending = {
        idx,
        entry,
        startX: ev.clientX,
        startY: ev.clientY,
        pointerId: ev.pointerId,
        armed: false,
      };
      try {
        stack.setPointerCapture(ev.pointerId);
      } catch (_) {
        /* ignore */
      }
    });

    stack.addEventListener("pointermove", (ev) => {
      if (!pending || ev.pointerId !== pending.pointerId) return;
      const dx = ev.clientX - pending.startX;
      const dy = ev.clientY - pending.startY;
      if (!pending.armed) {
        if (dx * dx + dy * dy < dragThresholdPx * dragThresholdPx) return;
        pending.armed = true;
        draggingHandIndex = pending.idx;
        pmEl.ignitionSelf?.classList.add("pm-drop-active");
        hideCardPreview();
        const wraps = [...stack.querySelectorAll(".pm-hand-card-wrap")];
        const wrapEl = wraps[pending.idx];
        const cardEl = wrapEl?.querySelector(".power-card");
        if (wrapEl && cardEl) {
          const rect = cardEl.getBoundingClientRect();
          const scale = rect.width / 220;
          pending.grabOffsetX = pending.startX - rect.left;
          pending.grabOffsetY = pending.startY - rect.top;
          pending.cardEl = cardEl;
          pending.wrapEl = wrapEl;
          wrapEl.style.minHeight = `${rect.height}px`;
          wrapEl.classList.add("pm-hand-card-wrap--dragging");
          document.body.appendChild(cardEl);
          cardEl.classList.add("power-card--hand-dragging");
          cardEl.style.position = "fixed";
          cardEl.style.left = `${rect.left}px`;
          cardEl.style.top = `${rect.top}px`;
          cardEl.style.width = "220px";
          cardEl.style.zIndex = "3000";
          cardEl.style.margin = "0";
          cardEl.style.pointerEvents = "none";
          const lift = 1.08;
          pending.dragScaleLifted = scale * lift;
          cardEl.style.transform = `scale(${pending.dragScaleLifted})`;
          cardEl.style.transformOrigin = "top left";
          stack.style.cursor = "grabbing";
        }
      } else if (pending.cardEl) {
        const s = pending.dragScaleLifted ?? 1;
        pending.cardEl.style.left = `${ev.clientX - (pending.grabOffsetX ?? 0)}px`;
        pending.cardEl.style.top = `${ev.clientY - (pending.grabOffsetY ?? 0)}px`;
        pending.cardEl.style.transform = `scale(${s})`;
      }
      const r = pmEl.ignitionSelf?.getBoundingClientRect();
      if (r && pending.armed && pmEl.ignitionSelf) {
        const x = ev.clientX;
        const y = ev.clientY;
        const over = x >= r.left && x <= r.right && y >= r.top && y <= r.bottom;
        pmEl.ignitionSelf.classList.toggle("pm-drop-hover", over);
      }
    });

    function finishPointer(ev) {
      if (!pending || ev.pointerId !== pending.pointerId) return;
      try {
        stack.releasePointerCapture(ev.pointerId);
      } catch (_) {
        /* ignore */
      }
      let droppedOnIgnition = false;
      if (pending.armed) {
        const r = pmEl.ignitionSelf?.getBoundingClientRect();
        let over = false;
        if (r) {
          const x = ev.clientX;
          const y = ev.clientY;
          over = x >= r.left && x <= r.right && y >= r.top && y <= r.bottom;
        }
        if (over) {
          droppedOnIgnition = true;
        }
      }
      if (droppedOnIgnition && isGameplayInputOpen()) {
        const idx = pending.idx;
        const droppedEntry = pending.entry;
        const droppedId = String(droppedEntry?.cardId || "");
        const droppedType = String(getCardDef(droppedId)?.type || "").toLowerCase();
        sendHandCardAction(lastSnapshot, idx);
        const enteredBanishMode = disruptionBanishFlow !== null;
        const enteredRetaliateMode = retaliateTargetFlow !== null;
        if (enteredBanishMode || enteredRetaliateMode) {
          // A follow-up selection mode entered: restore the dragged card to its slot instead of removing it.
          // finishPointer will re-render below (after clearDragChrome) to show the banish mode visual.
          resetHandDragVisual(pending);
        } else {
          if (pending.cardEl && pending.wrapEl) {
            const card = pending.cardEl;
            const wrap = pending.wrapEl;
            card.classList.remove("power-card--hand-dragging");
            wrap.classList.remove("pm-hand-card-wrap--dragging");
            wrap.style.minHeight = "";
            if (card.parentNode === document.body) {
              card.remove();
            }
          }
          // Only skip re-render for own-turn disruption (where the card legitimately moves to ignition).
          // In reaction windows, always re-render so the hand state is consistent.
          const skipStalePlaymat = droppedType === "disruption" && !lastSnapshot?.reactionWindow?.open;
          if (lastSnapshot && !skipStalePlaymat) {
            renderPlaymat(lastSnapshot);
          }
        }
        clearDragChrome();
        pending = null;
        if ((enteredBanishMode || enteredRetaliateMode) && lastSnapshot) {
          renderPlaymat(lastSnapshot);
        }
      } else {
        // Restore the hand card DOM when not committing to ignition (clear position:fixed).
        resetHandDragVisual(pending);
        clearDragChrome();
        pending = null;
      }
    }

    stack.addEventListener("pointerup", finishPointer);
    stack.addEventListener("pointercancel", finishPointer);
  }

  // ---------------------------------------------------------------------------
  // Playmat: drag-and-drop (hand card → ignition slot)
  // ---------------------------------------------------------------------------

  // Wire up the ignition slot as a drop target.
  function setupIgnitionDropTarget() {
    const slot = pmEl.ignitionSelf;
    if (!slot) return;

    slot.addEventListener("dragover", (ev) => {
      // Only allow drop when a hand card is being dragged.
      if (draggingHandIndex === null || !isGameplayInputOpen()) return;
      ev.preventDefault();
      ev.dataTransfer.dropEffect = "move";
    });

    slot.addEventListener("dragleave", (ev) => {
      // Ignore dragleave events that fire when entering a child element.
      if (slot.contains(ev.relatedTarget)) return;
      slot.classList.remove("pm-drop-hover");
    });

    slot.addEventListener("dragenter", (ev) => {
      if (draggingHandIndex === null || !isGameplayInputOpen()) return;
      ev.preventDefault();
      slot.classList.add("pm-drop-hover");
    });

    slot.addEventListener("drop", (ev) => {
      ev.preventDefault();
      if (!isGameplayInputOpen()) return;
      slot.classList.remove("pm-drop-hover");
      slot.classList.remove("pm-drop-active");
      const idx = draggingHandIndex;
      draggingHandIndex = null;
      if (idx === null) return;
      const dropId = String(hudForSeat(lastSnapshot, playerEl.value)?.hand?.[idx]?.cardId || "");
      const dropType = String(getCardDef(dropId)?.type || "").toLowerCase();
      sendHandCardAction(lastSnapshot, idx);
      if (lastSnapshot && retaliateTargetFlow !== null) {
        renderPlaymat(lastSnapshot);
        return;
      }
      if (lastSnapshot && dropType !== "disruption" && dropId !== "retaliate") {
        renderPlaymat(lastSnapshot);
      }
    });
  }

  setupIgnitionDropTarget();

  // Cancel first-pick step of Piece Swap on Escape (keeps the card in ignition).
  // Cancel disruption banish selection mode on Escape.
  document.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape") {
      if (igniteTargetFlow?.cardId === "piece-swap" && igniteTargetFlow.firstPick) {
        igniteTargetFlow.firstPick = null;
        if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
        return;
      }
      if (disruptionBanishFlow !== null) {
        disruptionBanishFlow = null;
        if (lastSnapshot) renderPlaymat(lastSnapshot);
        return;
      }
      if (retaliateTargetFlow !== null) {
        retaliateTargetFlow = null;
        if (lastSnapshot) renderPlaymat(lastSnapshot);
        return;
      }
      if (copiedZipLineSource !== null) {
        copiedZipLineSource = null;
        if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
        if (lastSnapshot) renderPlayerActionBanner(lastSnapshot);
      }
    }
  });

  // Prevent cards from being dropped anywhere outside the game shell.
  document.addEventListener("dragover", (ev) => {
    if (gameShellEl && !gameShellEl.contains(ev.target) && draggingHandIndex !== null) {
      ev.dataTransfer.dropEffect = "none";
    }
  });

  // DRAW button
  if (pmEl.drawBtn) {
    pmEl.drawBtn.addEventListener("click", () => {
      if (!isGameplayInputOpen()) return;
      send("draw_card", {});
    });
  }

  /** Keeps opponent HUD above the board and the local player below (mirrors board flip for Player B). */
  function syncBoardPerspectiveClass() {
    if (!boardWrapEl) return;
    boardWrapEl.classList.toggle("board-wrap-perspective-b", playerEl.value === "B");
  }

  function renderPlayerHud(snapshot) {
    const players = snapshot?.players || [];
    for (const p of players) {
      if (p.playerId === "A") {
        setBar(manaFillA, manaLabelA, p.mana, p.maxMana);
        setBar(energizedFillA, energizedLabelA, p.energizedMana, p.maxEnergized);
      } else if (p.playerId === "B") {
        setBar(manaFillB, manaLabelB, p.mana, p.maxMana);
        setBar(energizedFillB, energizedLabelB, p.energizedMana, p.maxEnergized);
      }
    }
  }

  /**
   * Blocks gameplay input during effect visuals. Overlapping calls extend the block to the
   * latest end time so rapid snapshots do not shorten the protection window.
   * @param {number} ms
   */
  function blockGameplayInputForEffects(ms) {
    const m = Math.max(0, Number(ms) || 0);
    effectAnimBlocking = true;
    const want = Date.now() + m;
    if (want > effectAnimUnblockAt) {
      effectAnimUnblockAt = want;
    }
    const hadScheduled = effectAnimBlockTimeout != null;
    if (effectAnimBlockTimeout) {
      clearTimeout(effectAnimBlockTimeout);
      effectAnimBlockTimeout = null;
    }
    if (!hadScheduled) {
      send("client_fx_hold", {});
    }
    const delay = Math.max(0, effectAnimUnblockAt - Date.now());
    effectAnimBlockTimeout = setTimeout(() => {
      effectAnimBlocking = false;
      effectAnimUnblockAt = 0;
      effectAnimBlockTimeout = null;
      send("client_fx_release", {});
    }, delay);
  }

  /**
   * Returns true when snapshot delta likely triggers card/effect visual transitions.
   * This prevents locking gameplay input on every periodic snapshot tick.
   * @param {object|null} prevSnap
   * @param {object} nextSnap
   * @returns {boolean}
   */
  function hasEffectAnimationDelta(prevSnap, nextSnap) {
    if (!prevSnap || !nextSnap) return false;
    if (ignitionHudSignature(prevSnap) !== ignitionHudSignature(nextSnap)) return true;
    if ((prevSnap.activationQueueSize || 0) !== (nextSnap.activationQueueSize || 0)) return true;
    const prevPE = Array.isArray(prevSnap.pendingEffects) ? prevSnap.pendingEffects.length : 0;
    const nextPE = Array.isArray(nextSnap.pendingEffects) ? nextSnap.pendingEffects.length : 0;
    if (prevPE !== nextPE) return true;
    const prevRW = prevSnap.reactionWindow || {};
    const nextRW = nextSnap.reactionWindow || {};
    if ((prevRW.stackSize || 0) !== (nextRW.stackSize || 0)) return true;
    if ((prevRW.stagedCardId || "") !== (nextRW.stagedCardId || "")) return true;
    if ((prevRW.stagedOwner || "") !== (nextRW.stagedOwner || "")) return true;
    return false;
  }

  /**
   * Whether the match allows gameplay input (not blocked by reconnect banner or local animations).
   * @param {object} [snapshot]
   * @param {{ turnStartAnimation?: boolean, effectAnimation?: boolean }} [uiLocks]
   */
  function isOpenGameState(snapshot, uiLocks) {
    if (snapshot?.gameStarted !== true || snapshot?.matchEnded) return false;
    if (uiLocks?.turnStartAnimation) return false;
    if (uiLocks?.effectAnimation) return false;
    if (snapshot?.reconnectPendingFor) return false;
    return true;
  }

  function isGameplayInputOpen() {
    return isOpenGameState(lastSnapshot, {
      turnStartAnimation: turnResourceAnimBlocking,
      effectAnimation: effectAnimBlocking,
    });
  }

  function isPromotionMove(from, to) {
    const code = lastSnapshot?.board?.[from.row]?.[from.col] || "";
    const piece = parseCode(code);
    return piece?.type === "P" && (to.row === 0 || to.row === 7);
  }

  function closePromotionModal(value) {
    if (!promotionOverlayEl) return;
    if (typeof promotionOverlayEl.close === "function" && promotionOverlayEl.open) {
      promotionOverlayEl.close();
    }
    promotionOverlayEl.classList.add("hidden");
    promotionOverlayEl.setAttribute("aria-hidden", "true");
    const resolve = promotionChoiceResolver;
    promotionChoiceResolver = null;
    if (resolve) resolve(value);
  }

  function requestPromotionChoice(from) {
    if (!promotionOverlayEl || !promotionChoicesEl) return Promise.resolve(null);
    if (promotionChoiceResolver) closePromotionModal(null);
    const code = lastSnapshot?.board?.[from.row]?.[from.col] || "";
    const color = code[0] === "b" ? "b" : "w";
    if (promotionTitleEl) promotionTitleEl.textContent = t("promotionTitle");
    if (promotionBodyEl) promotionBodyEl.textContent = t("promotionBody");
    if (promotionCancelEl) promotionCancelEl.textContent = t("cancel");
    promotionChoicesEl.innerHTML = "";
    for (const choice of promotionChoices) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "promotion-choice";
      btn.dataset.promotion = choice.value;
      const img = document.createElement("img");
      img.src = pieceImageURL(`${color}${choice.type}`);
      img.alt = t(choice.labelKey);
      const label = document.createElement("span");
      label.textContent = t(choice.labelKey);
      btn.appendChild(img);
      btn.appendChild(label);
      btn.addEventListener("click", () => closePromotionModal(choice.value));
      promotionChoicesEl.appendChild(btn);
    }
    promotionOverlayEl.classList.remove("hidden");
    promotionOverlayEl.setAttribute("aria-hidden", "false");
    if (typeof promotionOverlayEl.showModal === "function" && !promotionOverlayEl.open) {
      promotionOverlayEl.showModal();
    }
    promotionChoicesEl.querySelector("button")?.focus();
    return new Promise((resolve) => {
      promotionChoiceResolver = resolve;
    });
  }

  async function sendMove(from, to) {
    if (!isGameplayInputOpen()) return;
    const payload = {
      fromRow: from.row,
      fromCol: from.col,
      toRow: to.row,
      toCol: to.col,
    };
    if (isPromotionMove(from, to)) {
      const promotion = await requestPromotionChoice(from);
      if (!promotion || !isGameplayInputOpen()) return;
      payload.promotion = promotion;
    }
    send("submit_move", payload);
  }

  function pushClientTrace(entry) {
    try {
      clientTraceBuffer.push({ ts: new Date().toISOString(), ...entry });
      if (clientTraceBuffer.length > CLIENT_TRACE_CAP) {
        clientTraceBuffer.splice(0, clientTraceBuffer.length - CLIENT_TRACE_CAP);
      }
    } catch (_) {
      /* ignore */
    }
  }

  /**
   * Strips full boards / hands from trace entries before mirroring to server logs (ADMIN_DEBUG_MATCH).
   * The in-browser buffer keeps full payloads; use Download client trace for complete JSON.
   * @param {object} entry
   * @returns {object}
   */
  function slimTraceEntryForServerMirror(entry) {
    try {
      if (entry.envelope && typeof entry.envelope === "object") {
        const env = entry.envelope;
        const slim = { ts: entry.ts, dir: entry.dir, envelope: { type: env.type, id: env.id } };
        if (env.type === "state_snapshot" && env.payload && typeof env.payload === "object") {
          const p = env.payload;
          slim.envelope.payload = {
            roomId: p.roomId,
            turnPlayer: p.turnPlayer,
            turnNumber: p.turnNumber,
            matchEnded: p.matchEnded,
            pendingCapture: p.pendingCapture,
            reactionWindow: p.reactionWindow
              ? {
                  open: p.reactionWindow.open,
                  trigger: p.reactionWindow.trigger,
                  stackSize: p.reactionWindow.stackSize,
                }
              : undefined,
            mulliganPhaseActive: p.mulliganPhaseActive,
          };
        } else if (env.payload !== undefined) {
          slim.envelope.payload = env.payload;
        }
        return slim;
      }
      return { ts: entry.ts, dir: entry.dir, type: entry.type, payload: entry.payload };
    } catch (_) {
      return { ts: entry.ts, dir: entry.dir, note: "slim_failed" };
    }
  }

  function flushClientTraceToServer() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    if (!lastSnapshot?.adminDebugMatch || !joinedRoom) return;
    if (clientTraceBuffer.length === 0) return;
    const chunk = clientTraceBuffer.slice(-80).map(slimTraceEntryForServerMirror);
    let text;
    try {
      text = JSON.stringify(chunk);
    } catch (_) {
      text = `{"error":"serialize_failed"}`;
    }
    ws.send(JSON.stringify({ id: `req-${seq++}`, type: "client_trace", payload: { text } }));
  }

  function downloadClientTraceJson() {
    let raw;
    try {
      raw = JSON.stringify(clientTraceBuffer, null, 2);
    } catch (_) {
      raw = "[]";
    }
    const blob = new Blob([raw], { type: "application/json" });
    const a = document.createElement("a");
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    a.href = URL.createObjectURL(blob);
    a.download = `power-chess-client-trace-${stamp}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
  }

  function logEvent(obj) {
    try {
      pushClientTrace({ dir: "in", envelope: obj });
    } catch (_) {
      /* ignore */
    }
    if (!eventsEl) return;
    let line;
    try {
      line = JSON.stringify(obj, null, 2);
    } catch (_) {
      line = String(obj);
    }
    const cap = 32000;
    const next = `${line}\n\n${eventsEl.textContent}`;
    eventsEl.textContent = next.slice(0, cap);
  }

  function send(type, payload) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    if (type !== "client_trace") {
      try {
        pushClientTrace({ dir: "out", type, payload: payload ?? null });
      } catch (_) {
        /* ignore */
      }
    }
    const bypassTurnStartAnim =
      type === "ping" ||
      type === "pong" ||
      type === "leave_match" ||
      type === "client_trace" ||
      type === "client_fx_hold" ||
      type === "client_fx_release";
    if (turnResourceAnimBlocking && !bypassTurnStartAnim) return;
    ws.send(JSON.stringify({ id: `req-${seq++}`, type, payload }));
  }

  function makeEdgeLabel(text, classNames = []) {
    const el = document.createElement("div");
    el.className = "edge-label";
    for (const className of classNames) {
      if (className) el.classList.add(className);
    }
    el.textContent = text;
    return el;
  }

  function expectedActionPlayer(snapshot) {
    if (!snapshot || !gameStarted || snapshot.matchEnded || snapshot.mulliganPhaseActive) return "";
    if (snapshot.reconnectPendingFor) return "";
    if (Number(snapshot.connectedA ?? 1) <= 0 || Number(snapshot.connectedB ?? 1) <= 0) return "";

    if (snapshot.reactionWindow?.open) {
      return currentReactionResponder(snapshot);
    }
    const targeting = snapshot.ignitionTargeting;
    if (targeting?.awaitingTargetChoice && targeting.owner) {
      return String(targeting.owner);
    }
    if (Array.isArray(snapshot.pendingEffects) && snapshot.pendingEffects.length > 0) {
      const pendingOwner = snapshot.pendingEffects.find((pe) => pe?.owner)?.owner;
      if (pendingOwner) return String(pendingOwner);
    }
    return String(snapshot.turnPlayer || "");
  }

  function seatForLogicalHalf(row) {
    return row >= 4 ? "A" : "B";
  }

  function actionEdgeClasses(logicalRow, expectedSeat) {
    const sideSeat = seatForLogicalHalf(logicalRow);
    if (!expectedSeat || sideSeat !== expectedSeat) return [];
    const isOwnAction = expectedSeat === playerEl.value;
    return ["edge-label--action", isOwnAction ? "edge-label--action-own" : "edge-label--action-opponent"];
  }

  /** @returns {Set<string>} Keys for currently highlighted destination squares. */
  function boardMoveKeySet() {
    return new Set(highlightedMoves.map((m) => posKey(m.row, m.col)));
  }

  /**
   * Updates .selected / .move classes without rebuilding the board (keeps HTML5 drag alive).
   */
  function refreshBoardHighlights() {
    if (!boardFrameEl) return;
    const moveSet = boardMoveKeySet();
    const selectedKey = selectedFrom ? posKey(selectedFrom.row, selectedFrom.col) : null;
    const dotted = ignitionDottedPiecesForUI(lastSnapshot);
    const targetKeySet = new Set(dotted.map((tp) => posKey(tp.row, tp.col)));
    const zlSrc = viewerZipLinePendingSource(lastSnapshot);
    const zlDestSet = zlSrc ? zipLineDestKeySet(lastSnapshot?.board || [], zlSrc) : new Set();
    for (const sq of boardFrameEl.querySelectorAll(".sq[data-row]")) {
      const r = +sq.dataset.row;
      const c = +sq.dataset.col;
      sq.classList.toggle("selected", selectedKey === posKey(r, c));
      sq.classList.toggle("move", moveSet.has(posKey(r, c)));
      sq.classList.toggle("zip-line-dest", zlDestSet.has(posKey(r, c)));
      sq.classList.toggle("target-selected", targetKeySet.has(posKey(r, c)));
    }
  }

  /** Whether the local player may select or drag pieces on the chess board. */
  function canInteractChessPieces() {
    const s = lastSnapshot;
    if (!s?.board || !gameStarted || s.matchEnded) return false;
    if (s.mulliganPhaseActive) return false;
    if (!isGameplayInputOpen()) return false;
    if (s.turnPlayer !== playerEl.value) return false;
    if (viewerZipLinePendingSource(s)) return false;
    if (viewerPendingBoardTargetEffect(s)) return false;
    return true;
  }

  /** Draws arrow from attacker to capture target while `pendingCapture` is active (server snapshot). */
  function renderCaptureThreatOverlay() {
    const svg = captureThreatOverlayEl;
    if (!svg || !boardFrameEl) return;
    const pc = lastSnapshot?.pendingCapture;
    if (!pc?.active) {
      svg.innerHTML = "";
      return;
    }
    const paint = () => {
      if (!boardFrameEl || !captureThreatOverlayEl) return;
      const pcInner = lastSnapshot?.pendingCapture;
      if (!pcInner?.active) {
        svg.innerHTML = "";
        return;
      }
      const fr = Number(pcInner.fromRow ?? 0);
      const fc = Number(pcInner.fromCol ?? 0);
      const tr = Number(pcInner.toRow ?? 0);
      const tc = Number(pcInner.toCol ?? 0);
      const fromSq = boardFrameEl.querySelector(`.sq[data-row="${fr}"][data-col="${fc}"]`);
      const toSq = boardFrameEl.querySelector(`.sq[data-row="${tr}"][data-col="${tc}"]`);
      if (!fromSq || !toSq) {
        svg.innerHTML = "";
        return;
      }
      void boardFrameEl.offsetWidth;
      const br = boardFrameEl.getBoundingClientRect();
      const w = Math.max(1, br.width);
      const h = Math.max(1, br.height);
      svg.setAttribute("viewBox", `0 0 ${w} ${h}`);
      svg.setAttribute("width", "100%");
      svg.setAttribute("height", "100%");
      const r1 = fromSq.getBoundingClientRect();
      const r2 = toSq.getBoundingClientRect();
      const x1 = r1.left - br.left + r1.width / 2;
      const y1 = r1.top - br.top + r1.height / 2;
      const x2 = r2.left - br.left + r2.width / 2;
      const y2 = r2.top - br.top + r2.height / 2;
      const dx = x2 - x1;
      const dy = y2 - y1;
      const len = Math.hypot(dx, dy) || 1;
      const ux = dx / len;
      const uy = dy / len;
      const head = 12;
      const halfW = 5.5;
      const lineEndX = x2 - ux * head;
      const lineEndY = y2 - uy * head;
      const px = -uy;
      const py = ux;
      const b1x = lineEndX + px * halfW;
      const b1y = lineEndY + py * halfW;
      const b2x = lineEndX - px * halfW;
      const b2y = lineEndY - py * halfW;
      const line = `<line x1="${x1}" y1="${y1}" x2="${lineEndX}" y2="${lineEndY}" stroke="#dc2626" stroke-width="3" stroke-linecap="round" opacity="0.88"/>`;
      const headPoly = `<polygon points="${x2},${y2} ${b1x},${b1y} ${b2x},${b2y}" fill="#dc2626" opacity="0.88"/>`;
      svg.innerHTML = line + headPoly;
    };
    requestAnimationFrame(() => {
      requestAnimationFrame(paint);
    });
  }

  function renderBoard(board) {
    if (!boardFrameEl) return;
    syncBoardPerspectiveClass();
    if (pmPreviewCard && boardFrameEl.contains(pmPreviewCard)) {
      hideCardPreview();
    }
    boardEnchantHoverSuppressed = false;
    boardFrameEl.innerHTML = "";
    boardFrameEl.classList.toggle("show-inner-coords", coordsInSquaresEl && coordsInSquaresEl.checked);
    boardFrameEl.classList.toggle(
      "effect-duration-badges-hover-only",
      !!(effectTurnsAlwaysVisibleEl && !effectTurnsAlwaysVisibleEl.checked),
    );
    const moveSet = boardMoveKeySet();
    const selectedKey = selectedFrom ? posKey(selectedFrom.row, selectedFrom.col) : null;
    const dotted = ignitionDottedPiecesForUI(lastSnapshot);
    const targetKeySet = new Set(dotted.map((tp) => posKey(tp.row, tp.col)));
    const ep = lastSnapshot?.enPassant;
    const pendingCap = lastSnapshot?.pendingCapture;
    const capToR = Number(pendingCap?.toRow ?? -1);
    const capToC = Number(pendingCap?.toCol ?? -1);

    // Precompute Piece Swap targeting highlights.
    const psOwnColor = playerEl.value === "A" ? "w" : "b";
    const psFp =
      igniteTargetFlow?.cardId === "piece-swap" && igniteTargetFlow.stage === "picking"
        ? igniteTargetFlow.firstPick || null
        : null;
    const isPsPickingOwn =
      igniteTargetFlow?.cardId === "piece-swap" && igniteTargetFlow.stage === "picking" && !igniteTargetFlow.firstPick;
    const psEligibleSet = isPsPickingOwn ? pieceSwapEligibleOwnSet(board || [], psOwnColor) : null;
    const psValidTargetSet = psFp
      ? new Set(pieceSwapValidTargets(board || [], psFp, psOwnColor).map((p) => posKey(p.row, p.col)))
      : null;

    const zlSrc = viewerZipLinePendingSource(lastSnapshot);
    const zlDestSet = zlSrc ? zipLineDestKeySet(board || [], zlSrc) : new Set();
    const actionSeat = expectedActionPlayer(lastSnapshot);

    for (let gr = 0; gr < 10; gr++) {
      for (let gc = 0; gc < 10; gc++) {
        const corner = (gr === 0 || gr === 9) && (gc === 0 || gc === 9);
        if (corner) {
          const cornerEl = document.createElement("div");
          cornerEl.className = "edge-label";
          boardFrameEl.appendChild(cornerEl);
          continue;
        }

        if (gr === 0 && gc >= 1 && gc <= 8) {
          const dc = gc - 1;
          const { row } = displayToLogical(0, dc);
          boardFrameEl.appendChild(makeEdgeLabel(fileLetterFromDisplayEdge(0, dc), actionEdgeClasses(row, actionSeat)));
          continue;
        }
        if (gr === 9 && gc >= 1 && gc <= 8) {
          const dc = gc - 1;
          const { row } = displayToLogical(7, dc);
          boardFrameEl.appendChild(makeEdgeLabel(fileLetterFromDisplayEdge(7, dc), actionEdgeClasses(row, actionSeat)));
          continue;
        }
        if (gc === 0 && gr >= 1 && gr <= 8) {
          const dr = gr - 1;
          const { row } = displayToLogical(dr, 0);
          boardFrameEl.appendChild(makeEdgeLabel(rankDigitFromDisplayEdge(dr, 0), actionEdgeClasses(row, actionSeat)));
          continue;
        }
        if (gc === 9 && gr >= 1 && gr <= 8) {
          const dr = gr - 1;
          const { row } = displayToLogical(dr, 7);
          boardFrameEl.appendChild(makeEdgeLabel(rankDigitFromDisplayEdge(dr, 7), actionEdgeClasses(row, actionSeat)));
          continue;
        }

        const dr = gr - 1;
        const dc = gc - 1;
        const logical = displayToLogical(dr, dc);
        const r = logical.row;
        const c = logical.col;
        const sq = document.createElement("div");
        const code = board?.[r]?.[c] || "";
        sq.className = `sq grid-cell ${(dr + dc) % 2 === 0 ? "light" : "dark"}`;
        if (code) sq.classList.add("piece");
        if (selectedKey === posKey(r, c)) sq.classList.add("selected");
        if (moveSet.has(posKey(r, c))) sq.classList.add("move");
        if (zlDestSet.has(posKey(r, c))) sq.classList.add("zip-line-dest");
        if (targetKeySet.has(posKey(r, c))) sq.classList.add("target-selected");
        // Piece Swap two-step targeting highlights.
        if (psEligibleSet?.has(posKey(r, c))) {
          sq.classList.add("ps-eligible");
        } else if (psFp) {
          if (psFp.row === r && psFp.col === c) {
            sq.classList.add("ps-own-selected");
          } else if (chebyshevDist(psFp, { row: r, col: c }) <= 2) {
            sq.classList.add("ps-range");
            if (psValidTargetSet?.has(posKey(r, c))) sq.classList.add("ps-target");
          }
        }
        const auraFxArr = code ? pieceActivePowerAuras(lastSnapshot, r, c) : [];
        const hasMindControlDebuff = auraFxArr.some((e) => String(e.cardId || "") === "mind-control");
        // Double Turn grants an extra move to all pieces of the affected player.
        // The highlight is visible to both players: check piece color vs seat, not isOwnPiece.
        const dtSeat = lastSnapshot?.doubleTurnActiveFor; // "A" | "B" | undefined
        const isDoubleTurnPiece = !!(code && dtSeat && seatForPieceCode(code) === dtSeat);
        const hasAura = auraFxArr.length > 0 || isDoubleTurnPiece;
        if (hasAura) {
          sq.classList.add("piece-effect-aura-power");
        }
        if (hasMindControlDebuff) {
          sq.classList.add("piece-effect-outline-mind-control");
        }
        // Opponent can hover buffed enemy pieces too (same preview as owner); cursor: help via CSS.
        if (hasAura && code && !isOwnPiece(code)) {
          sq.classList.add("sq--foreign-buff");
        }
        if (pendingCap?.active && capToR === r && capToC === c) {
          sq.classList.add("capture-threat-target");
        }
        const coordSpan = document.createElement("span");
        coordSpan.className = "sq-coord";
        coordSpan.textContent = logicalToAlgebraic(r, c);
        sq.appendChild(coordSpan);
        if (code) {
          const img = document.createElement("img");
          img.className = "piece-img";
          img.src = pieceImageURL(code);
          img.alt = code;
          img.draggable = false;
          sq.appendChild(img);
        }
        // Badge shows the minimum turnsRemaining across all active effects on this piece.
        // Double Turn is included alongside per-piece effects (uses doubleTurnTurnsRemaining).
        if (hasAura) {
          const dtTurns = isDoubleTurnPiece ? (lastSnapshot?.doubleTurnTurnsRemaining ?? 1) : Infinity;
          const perPieceTurns =
            auraFxArr.length > 0
              ? auraFxArr.reduce((min, e) => Math.min(min, Math.max(Number(e.turnsRemaining || 0), 0)), Infinity)
              : Infinity;
          const minTurns = Math.min(dtTurns, perPieceTurns);
          if (isFinite(minTurns) && minTurns > 0) {
            const turnBadge = document.createElement("span");
            turnBadge.className = "sq-effect-turn-badge";
            turnBadge.textContent = `${minTurns}t`;
            sq.appendChild(turnBadge);
          }
        }
        sq.title = code ? `${code} ${logicalToAlgebraic(r, c)}` : logicalToAlgebraic(r, c);
        sq.dataset.row = String(r);
        sq.dataset.col = String(c);
        sq.dataset.code = code;
        sq.draggable = !!(code && isOwnPiece(code) && canInteractChessPieces());

        if (hasAura && code) {
          sq.addEventListener("mouseenter", () => {
            if (boardEnchantHoverSuppressed) return;
            // Buff preview is shown for both seats: local player and opponent see the same
            // card stack (activePieceEffects + double-turn when applicable).
            // Build list of effects to display: per-piece effects first, then Double Turn.
            const effectsList = auraFxArr
              .map((e) => ({
                cardData: cardDataFromCatalogRow(getCardDef(e.cardId)),
                turnsRemaining: Math.max(Number(e.turnsRemaining || 0), 0),
              }))
              .filter((e) => e.cardData != null);
            if (isDoubleTurnPiece) {
              const dtData = cardDataFromCatalogRow(getCardDef("double-turn"));
              if (dtData) {
                const dtTurns = lastSnapshot?.doubleTurnTurnsRemaining ?? 1;
                effectsList.push({ cardData: dtData, turnsRemaining: dtTurns });
              }
            }
            if (effectsList.length > 0) showPieceEffectsPreview(effectsList, sq);
          });
          sq.addEventListener("mousemove", () => {
            if (pmPreviewCard === sq) positionCardPreview(sq);
          });
          sq.addEventListener("mouseleave", () => {
            if (pmPreviewCard === sq) hideCardPreview();
            boardEnchantHoverSuppressed = false;
          });
          sq.addEventListener("mousedown", () => {
            if (!isOwnPiece(code) || !canInteractChessPieces()) return;
            boardEnchantHoverSuppressed = true;
            hideCardPreview();
          });
        }

        sq.addEventListener("click", () => {
          if (!lastSnapshot?.board || !gameStarted) return;
          const pendingBoardTarget = viewerPendingBoardTargetEffect(lastSnapshot);
          if (pendingBoardTarget && isGameplayInputOpen()) {
            const targetCode = sq.dataset.code || "";
            if (pendingBoardTarget.cardId === "mind-control") {
              if (!targetCode || isOwnPiece(targetCode)) return;
              const p = parseCode(targetCode);
              if (!p || p.type === "K" || p.type === "Q") return;
              send("resolve_pending_effect", { pieceRow: r, pieceCol: c });
              return;
            }
            if (pendingBoardTarget.cardId === "sacrifice-of-the-masses") {
              if (!targetCode || !isOwnPiece(targetCode)) return;
              const p = parseCode(targetCode);
              if (!p || p.type !== "P") return;
              send("resolve_pending_effect", { pieceRow: r, pieceCol: c });
              return;
            }
            if (!targetCode || !isOwnPiece(targetCode)) return;
            if (pendingBoardTarget.cardId === "zip-line") {
              const p = parseCode(targetCode);
              if (!p || p.type === "K") return;
              copiedZipLineSource = { row: r, col: c };
              renderBoard(lastSnapshot.board);
              renderPlayerActionBanner(lastSnapshot);
              return;
            }
            send("resolve_pending_effect", { pieceRow: r, pieceCol: c });
            return;
          }
          const zlPick = viewerZipLinePendingSource(lastSnapshot);
          if (zlPick && isGameplayInputOpen()) {
            const zd = zipLineDestKeySet(lastSnapshot.board, zlPick);
            if (zd.has(posKey(r, c)) && !code) {
              if (copiedZipLineSource) {
                send("resolve_pending_effect", {
                  pieceRow: copiedZipLineSource.row,
                  pieceCol: copiedZipLineSource.col,
                  destRow: r,
                  destCol: c,
                });
                copiedZipLineSource = null;
              } else {
                send("resolve_pending_effect", { destRow: r, destCol: c });
              }
              return;
            }
          }
          if (igniteTargetFlow?.stage === "picking") {
            const ownColor = playerEl.value === "A" ? "w" : "b";
            if (igniteTargetFlow.cardId === "piece-swap") {
              if (!igniteTargetFlow.firstPick) {
                // Stage 1: select own non-king piece with at least one valid target.
                const code = sq.dataset.code || "";
                if (!code || !isOwnPiece(code)) return;
                const p = parseCode(code);
                if (!p || p.type === "K") return;
                if (pieceSwapValidTargets(lastSnapshot.board, { row: r, col: c }, ownColor).length === 0) return;
                igniteTargetFlow.firstPick = { row: r, col: c };
                renderBoard(lastSnapshot.board);
                return;
              }
              // Stage 2: select valid opponent piece in range.
              const code = sq.dataset.code || "";
              if (!code || isOwnPiece(code)) return;
              const p = parseCode(code);
              if (!p || p.type === "K") return;
              const fp = igniteTargetFlow.firstPick;
              if (chebyshevDist(fp, { row: r, col: c }) > 2) return;
              if (isKingInCheckAfterSwap(lastSnapshot.board, fp, { row: r, col: c }, ownColor)) return;
              send("submit_ignition_targets", {
                target_pieces: [
                  { row: fp.row, col: fp.col },
                  { row: r, col: c },
                ],
              });
              igniteTargetFlow = null;
              return;
            }
            if (igniteTargetFlow.cardId === "mind-control") {
              const targetCode = sq.dataset.code || "";
              if (!targetCode || isOwnPiece(targetCode)) return;
              const p = parseCode(targetCode);
              if (!p || p.type === "K" || p.type === "Q") return;
              send("submit_ignition_targets", {
                target_pieces: [{ row: r, col: c }],
              });
              igniteTargetFlow = null;
              return;
            }
            if (igniteTargetFlow.cardId === "sacrifice-of-the-masses") {
              const targetCode = sq.dataset.code || "";
              if (!targetCode || !isOwnPiece(targetCode)) return;
              const p = parseCode(targetCode);
              if (!p || p.type !== "P") return;
              send("submit_ignition_targets", {
                target_pieces: [{ row: r, col: c }],
              });
              igniteTargetFlow = null;
              return;
            }
            // Default single-target handling (Knight/Bishop/Rook Touch).
            const clickedCodeForTarget = sq.dataset.code || "";
            if (!clickedCodeForTarget || !isOwnPiece(clickedCodeForTarget)) return;
            send("submit_ignition_targets", {
              target_pieces: [{ row: r, col: c }],
            });
            igniteTargetFlow = null;
            return;
          }
          const clickedCode = sq.dataset.code || "";
          if (clickedCode && isOwnPiece(clickedCode) && canInteractChessPieces()) {
            boardEnchantHoverSuppressed = true;
            hideCardPreview();
            selectedFrom = logical;
            highlightedMoves = computeMoves(lastSnapshot.board, selectedFrom, ep, lastSnapshot?.castlingRights);
            renderBoard(lastSnapshot.board);
            renderPlayerActionBanner(lastSnapshot);
            return;
          }
          const destSet = boardMoveKeySet();
          if (selectedFrom && destSet.has(posKey(r, c))) {
            sendMove(selectedFrom, logical);
            selectedFrom = null;
            highlightedMoves = [];
            renderBoard(lastSnapshot.board);
            renderPlayerActionBanner(lastSnapshot);
          }
        });

        sq.addEventListener("dragstart", (ev) => {
          const dragCode = sq.dataset.code || "";
          if (!canInteractChessPieces() || !dragCode || !isOwnPiece(dragCode)) {
            ev.preventDefault();
            return;
          }
          boardEnchantHoverSuppressed = true;
          hideCardPreview();
          const dt = ev.dataTransfer;
          if (!dt) {
            ev.preventDefault();
            return;
          }
          draggingFrom = logical;
          selectedFrom = logical;
          highlightedMoves = computeMoves(
            lastSnapshot?.board,
            draggingFrom,
            lastSnapshot?.enPassant,
            lastSnapshot?.castlingRights,
          );
          refreshBoardHighlights();
          renderPlayerActionBanner(lastSnapshot);
          const pImg = sq.querySelector(".piece-img");
          if (pImg instanceof HTMLImageElement) {
            const w = pImg.offsetWidth || 48;
            const h = pImg.offsetHeight || 48;
            dt.setDragImage(pImg, w / 2, h / 2);
          }
          // Add the fading class AFTER setDragImage so the browser captures the
          // ghost at full opacity (the class reduces piece opacity to 0.35).
          sq.classList.add("sq--piece-dragging");
          dt.setData("text/plain", posKey(r, c));
          dt.effectAllowed = "move";
        });

        sq.addEventListener("dragover", (ev) => {
          if (!draggingFrom) return;
          if (!boardMoveKeySet().has(posKey(r, c))) return;
          ev.preventDefault();
          sq.classList.add("drop-target");
          ev.dataTransfer.dropEffect = "move";
        });

        sq.addEventListener("dragenter", (ev) => {
          if (!draggingFrom) return;
          if (!boardMoveKeySet().has(posKey(r, c))) return;
          ev.preventDefault();
        });

        sq.addEventListener("dragleave", () => {
          sq.classList.remove("drop-target");
        });

        sq.addEventListener("drop", (ev) => {
          sq.classList.remove("drop-target");
          if (!draggingFrom || !boardMoveKeySet().has(posKey(r, c))) return;
          ev.preventDefault();
          const from = draggingFrom;
          const to = logical;
          draggingFrom = null;
          selectedFrom = null;
          highlightedMoves = [];
          sendMove(from, to);
          if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
          renderPlayerActionBanner(lastSnapshot);
        });

        sq.addEventListener("dragend", () => {
          boardFrameEl?.querySelectorAll(".sq--piece-dragging").forEach((el) => {
            el.classList.remove("sq--piece-dragging");
          });
          draggingFrom = null;
          selectedFrom = null;
          highlightedMoves = [];
          if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
          renderPlayerActionBanner(lastSnapshot);
        });

        boardFrameEl.appendChild(sq);
      }
    }
    renderCaptureThreatOverlay();
  }

  function renderStatus(snapshot) {
    if (!statusEl) return;
    const rw = snapshot?.reactionWindow || {};
    const pc = snapshot?.pendingCapture || {};
    statusEl.textContent = JSON.stringify(
      {
        pendingCapture: pc,
        reactionWindow: rw,
        pendingEffects: snapshot?.pendingEffects || [],
      },
      null,
      2,
    );
  }

  /** @type {ReturnType<typeof setInterval> | null} */
  let opponentDisconnectInterval = null;

  function clearOpponentDisconnectInterval() {
    if (opponentDisconnectInterval) {
      clearInterval(opponentDisconnectInterval);
      opponentDisconnectInterval = null;
    }
  }

  function hideOpponentDisconnectOverlay() {
    if (!opponentDisconnectOverlayEl) return;
    opponentDisconnectOverlayEl.classList.add("hidden");
    opponentDisconnectOverlayEl.setAttribute("aria-hidden", "true");
    clearOpponentDisconnectInterval();
  }

  /**
   * Shows a countdown while the opponent's reconnect grace timer is running (server closes the seat to new joins).
   * @param {object} payload state_snapshot
   */
  function updateOpponentDisconnectOverlay(payload) {
    if (!opponentDisconnectOverlayEl) return;
    const pending = payload?.reconnectPendingFor;
    const deadline = Number(payload?.reconnectDeadlineUnixMs || 0);
    const local = playerEl.value;
    const show = payload && !payload.matchEnded && pending && pending !== local && deadline > 0;
    if (!show) {
      hideOpponentDisconnectOverlay();
      return;
    }
    opponentDisconnectOverlayEl.classList.remove("hidden");
    opponentDisconnectOverlayEl.setAttribute("aria-hidden", "false");
    const titleEl = document.getElementById("opponentDisconnectTitle");
    const countdownEl = document.getElementById("opponentDisconnectCountdown");
    document.getElementById("opponentDisconnectHint").textContent = t("opponentDisconnectedHint");
    if (countdownEl) countdownEl.classList.add("hidden");
    const tick = () => {
      const sec = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      if (titleEl) titleEl.textContent = t("opponentDisconnectedBanner", { s: sec });
      if (sec <= 0) {
        hideOpponentDisconnectOverlay();
      }
    };
    tick();
    clearOpponentDisconnectInterval();
    opponentDisconnectInterval = setInterval(tick, 500);
  }

  function hideMatchEndOverlay() {
    if (!matchEndOverlayEl) return;
    matchEndOverlayEl.classList.add("hidden");
    matchEndOverlayEl.setAttribute("aria-hidden", "true");
    if (matchEndRematchEl) {
      matchEndRematchEl.classList.add("hidden");
      matchEndRematchEl.disabled = false;
    }
    if (matchEndStayEl) matchEndStayEl.classList.add("hidden");
    if (matchEndCountdownEl) {
      matchEndCountdownEl.classList.add("hidden");
      matchEndCountdownEl.textContent = "";
    }
    if (matchCountdownTimer) {
      clearInterval(matchCountdownTimer);
      matchCountdownTimer = null;
    }
  }

  function showMatchEndOverlay(title, bodyText) {
    if (!matchEndOverlayEl) return;
    const titleEl = document.getElementById("matchEndTitle");
    if (titleEl) titleEl.textContent = title;
    if (matchEndBodyEl) matchEndBodyEl.textContent = bodyText;
    matchEndOverlayEl.classList.remove("hidden");
    matchEndOverlayEl.setAttribute("aria-hidden", "false");
  }

  function describeEndReason(code) {
    const m = {
      checkmate: t("reasonCheckmate"),
      stalemate: t("reasonStalemate"),
      both_disconnected_cancelled: t("reasonBothDisconnected"),
      disconnect_timeout: t("reasonDisconnectTimeout"),
      left_room: t("reasonLeftRoom"),
    };
    return m[code] || (code ? `${t("reasonPrefix")}: ${code}.` : "");
  }

  function buildMatchEndMessage(payload) {
    const you = playerEl.value;
    const w = payload.winner;
    if (payload.endReason === "left_room" && w === you) {
      return `${t("youWon")}\n\n${t("reasonOpponentAbandoned")}`;
    }
    if (payload.endReason === "checkmate") {
      if (w === you) return `${t("youWon")}\n\n${t("reasonCheckmateShort")}`;
      return `${t("youLost")}\n\n${t("reasonCheckmateShort")}`;
    }
    if (payload.endReason === "stalemate") {
      return `${t("draw")}\n\n${t("reasonStalemateShort")}`;
    }
    let headline = t("matchEndedNoWinner");
    if (w === you) headline = t("youWon");
    else if (w === "A" || w === "B") headline = t("youLost");
    const reasonLine = describeEndReason(payload.endReason || "");
    return `${headline}\n\n${reasonLine}`.trim();
  }

  function localRematchVote(payload) {
    return playerEl.value === "A" ? !!payload.rematchA : !!payload.rematchB;
  }

  function opponentRematchVote(payload) {
    return playerEl.value === "A" ? !!payload.rematchB : !!payload.rematchA;
  }

  function buildPostMatchModalMessage(payload) {
    const base = buildMatchEndMessage(payload);
    const connected = (payload.connectedA || 0) + (payload.connectedB || 0);
    const localVotedRematch = localRematchVote(payload);
    const opponentVotedRematch = opponentRematchVote(payload);
    if (connected === 1 && localVotedRematch) {
      return `${base}\n\n${t("rematchOpponentLeft")}`;
    }
    if (connected === 2 && opponentVotedRematch && !localVotedRematch) {
      return `${base}\n\n${t("rematchProposed")}`;
    }
    if (connected === 2 && localVotedRematch && !opponentVotedRematch) {
      return `${base}\n\n${t("rematchWaiting")}`;
    }
    return base;
  }

  function maybeShowMatchEndModal(payload) {
    if (!payload.matchEnded) {
      prevMatchEnded = false;
      hideMatchEndOverlay();
      return;
    }
    updatePostMatchActionControls(payload);
    if (!prevMatchEnded) {
      hideOpponentDisconnectOverlay();
      showMatchEndOverlay(t("matchFinished"), buildPostMatchModalMessage(payload));
    } else if (matchEndOverlayEl && !matchEndOverlayEl.classList.contains("hidden")) {
      if (matchEndBodyEl) matchEndBodyEl.textContent = buildPostMatchModalMessage(payload);
    }
    prevMatchEnded = true;
  }

  function updatePostMatchActionControls(payload) {
    if (!matchEndRematchEl || !matchEndStayEl) return;
    const connected = (payload.connectedA || 0) + (payload.connectedB || 0);
    const ended = payload.matchEnded === true;
    const localVotedRematch = localRematchVote(payload);
    matchEndRematchEl.classList.toggle("hidden", !ended || connected !== 2);
    matchEndRematchEl.disabled = !ended || connected !== 2 || localVotedRematch;
    matchEndStayEl.classList.toggle("hidden", connected !== 1);
    startPostMatchCountdown(payload);
  }

  function startPostMatchCountdown(payload) {
    if (matchCountdownTimer) {
      clearInterval(matchCountdownTimer);
      matchCountdownTimer = null;
    }
    if (!matchEndCountdownEl) return;
    function tick() {
      if (!payload?.matchEnded) {
        matchEndCountdownEl.classList.add("hidden");
        return;
      }
      const ms = Number(payload.postMatchMsLeft || 0);
      const s = Math.max(0, Math.ceil(ms / 1000));
      matchEndCountdownEl.textContent = s > 0 ? t("autoCloseIn", { s }) : t("autoCloseNow");
      matchEndCountdownEl.classList.remove("hidden");
      if (s <= 0 && matchCountdownTimer) {
        clearInterval(matchCountdownTimer);
        matchCountdownTimer = null;
      }
    }
    tick();
    matchCountdownTimer = setInterval(() => {
      if (!lastSnapshot?.matchEnded) {
        if (matchCountdownTimer) clearInterval(matchCountdownTimer);
        matchCountdownTimer = null;
        matchEndCountdownEl.classList.add("hidden");
        return;
      }
      const next = {
        ...lastSnapshot,
        postMatchMsLeft: Math.max(0, Number(lastSnapshot.postMatchMsLeft || 0) - 1000),
      };
      lastSnapshot = next;
      tick();
    }, 1000);
  }

  function updateLobbyChromeFromSnapshot(payload) {
    const started = payload.gameStarted === true;
    const ended = payload.matchEnded === true;
    gameStarted = started;
    if (waitingBannerEl) waitingBannerEl.classList.toggle("hidden", started || ended);
    if (boardAreaEl) boardAreaEl.classList.remove("hidden");
    renderInRoomLabel(payload);
  }

  function renderInRoomLabel(payload) {
    if (!inRoomLabelEl) return;
    const roomName = payload.roomName || "Let's Play!";
    const roomId = payload.roomId || "";
    const privacy = payload.roomPrivate ? t("private") : t("public");
    inRoomLabelEl.textContent = "";
    inRoomLabelEl.append(`${t("room")}: ${roomName} (#${roomId}) | ${privacy}`);
    if (payload.roomPrivate) {
      const raw = String(payload.roomPassword || "");
      const masked = raw ? "•".repeat(Math.max(raw.length, 4)) : "••••";
      const value = revealRoomPassword ? raw : masked;
      const toggle = document.createElement("button");
      toggle.type = "button";
      toggle.className = "inline-link-btn";
      toggle.textContent = revealRoomPassword ? t("hide") : t("show");
      toggle.addEventListener("click", () => {
        revealRoomPassword = !revealRoomPassword;
        renderInRoomLabel(payload);
      });
      inRoomLabelEl.append(` | ${t("passwordLabelInline")}: `, value, " (", toggle, ")");
    }
    const selfRoom =
      authUser && authUser.username && String(authUser.username).trim()
        ? String(authUser.username).trim()
        : `${t("player")} ${playerEl.value}`;
    inRoomLabelEl.append(` | ${selfRoom}`);
  }

  /**
   * Sends one `debug_match_fixture` on the snapshot where the second player has just joined
   * (transition to both connected). Optionally queues mulligan confirm for a later snapshot.
   * Enable via `match-test-config.js` or console: `__powerChessMatchTest.autoApply = true` (this tab).
   * @param {object} payload state_snapshot
   */
  function maybeApplyMatchTestFixture(payload) {
    if (!payload || payload.matchEnded) return;
    const both = Number(payload.connectedA) > 0 && Number(payload.connectedB) > 0;

    if (matchTestAutoConfirmMulliganEnabled() && matchTestAwaitingMulliganConfirm && payload.mulliganPhaseActive) {
      const mr = payload.mulliganReturned || {};
      const my = playerEl.value;
      if (mr[my] === undefined || mr[my] < 0) {
        matchTestAwaitingMulliganConfirm = false;
        send("confirm_mulligan", { handIndices: [] });
      }
    }

    if (matchTestAutoApplyEnabled() && !matchTestFixtureSent && both && !matchTestPrevBothConnected) {
      matchTestFixtureSent = true;
      send("debug_match_fixture", buildMatchDebugFixturePayload());
      if (matchTestAutoConfirmMulliganEnabled()) {
        matchTestAwaitingMulliganConfirm = true;
      }
    }

    matchTestPrevBothConnected = both;
  }

  function resetToLobbyUi() {
    if (clientTraceMirrorTimerId) {
      clearInterval(clientTraceMirrorTimerId);
      clientTraceMirrorTimerId = null;
    }
    joinedRoom = false;
    gameStarted = false;
    matchTestFixtureSent = false;
    matchTestPrevBothConnected = false;
    matchTestAwaitingMulliganConfirm = false;
    lastSnapshot = null;
    pmPrevSnapshot = null;
    prevReceivedSnapshot = null;
    snapshotApplyChain = Promise.resolve();
    setLobbyFooterVisible(true);
    if (lobbyScreenEl) lobbyScreenEl.classList.remove("hidden");
    if (gameShellEl) gameShellEl.classList.add("hidden");
    if (playerEl) playerEl.disabled = false;
    if (roomNameEl) roomNameEl.disabled = false;
    if (roomSearchEl) roomSearchEl.disabled = false;
    if (privateRoomEl) privateRoomEl.disabled = false;
    if (roomPasswordEl) roomPasswordEl.disabled = false;
    if (pieceTypeEl) pieceTypeEl.disabled = false;
    if (waitingBannerEl) waitingBannerEl.classList.add("hidden");
    if (boardAreaEl) boardAreaEl.classList.remove("hidden");
    renderBoard([]);
    renderStatus({});
    if (snapshotEl) snapshotEl.textContent = "";
    if (inRoomLabelEl) inRoomLabelEl.textContent = "";
    revealRoomPassword = false;
    prevMatchEnded = false;
    pendingActivateCardPayloads = [];
    activationFxQueue = [];
    activationFxWorkerPromise = null;
    hideMatchEndOverlay();
    hideOpponentDisconnectOverlay();
    updateReactionPassButton(null);
    hideLobbyPrivatePasswordError();
    startRoomListPolling();
    void refreshLobbyDecks();
  }

  function renderRoomList(rooms) {
    if (!roomListEl || !roomListEmptyEl) return;
    roomListEl.innerHTML = "";
    if (!rooms.length) {
      roomListEmptyEl.classList.remove("hidden");
      return;
    }
    roomListEmptyEl.classList.add("hidden");
    for (const rm of rooms) {
      const li = document.createElement("li");
      li.className = "room-list-item";
      const occ = (rm.connectedA || 0) + (rm.connectedB || 0);
      const label = rm.gameStarted ? t("statusPlaying") : t("statusWaiting");
      const occupiedBy = occ === 1 && rm.occupiedByColor ? ` | ${rm.occupiedByColor}` : "";
      const lock = rm.roomPrivate
        ? '<img class="room-lock-icon" src="/public/lock-keyhole.png" alt="Sala privada" title="Sala privada">'
        : "";
      const roomName = rm.roomName || "Let's Play!";
      li.innerHTML = `<span class="room-item-name">${lock}${roomName}</span> — ${occ}/2 (${label}${occupiedBy})`;
      li.addEventListener("click", () => {
        void (async () => {
          roomNameEl.value = rm.roomName || "Let's Play!";
          privateRoomEl.checked = false;
          updatePrivatePasswordVisibility();
          const pieceType = pieceTypeForRoomJoin(rm);
          let joinPassword = "";
          if (rm.roomPrivate) {
            const typed = await showPrivateJoinModal(rm.roomName || "Let's Play!");
            if (typed == null) return;
            joinPassword = typed;
          }
          void connectToRoom(rm.roomId, pieceType, rm.roomName || "Let's Play!", !!rm.roomPrivate, joinPassword);
        })();
      });
      roomListEl.appendChild(li);
    }
  }

  function applyRoomSearch() {
    const q = (roomSearchEl.value || "").trim().toLowerCase();
    if (!q) {
      renderRoomList(lobbyRooms);
      return;
    }
    const filtered = lobbyRooms.filter(
      (rm) =>
        String(rm.roomId || "")
          .toLowerCase()
          .includes(q) ||
        String(rm.roomName || "")
          .toLowerCase()
          .includes(q),
    );
    renderRoomList(filtered);
  }

  function pieceTypeForRoomJoin(rm) {
    if ((rm.connectedA || 0) > 0 && (rm.connectedB || 0) === 0) return "black";
    if ((rm.connectedB || 0) > 0 && (rm.connectedA || 0) === 0) return "white";
    return (pieceTypeEl && pieceTypeEl.value) || "random";
  }

  async function refreshRoomList() {
    if (joinedRoom) return;
    try {
      const r = await fetch("/api/rooms");
      if (!r.ok) return;
      const data = await r.json();
      lobbyRooms = data.rooms || [];
      applyRoomSearch();
    } catch (_) {
      /* lobby optional */
    }
  }

  function startRoomListPolling() {
    if (roomListTimer) return;
    refreshRoomList();
    roomListTimer = setInterval(refreshRoomList, 4000);
  }

  function stopRoomListPolling() {
    if (roomListTimer) {
      clearInterval(roomListTimer);
      roomListTimer = null;
    }
  }

  // -----------------------------------------------------------------------
  // Lobby UI event listeners
  // -----------------------------------------------------------------------
  document.getElementById("connectBtn").addEventListener("click", () => {
    void connectToRoom("", pieceTypeEl.value, roomNameEl.value);
  });
  if (lobbyDeckSelectEl) {
    lobbyDeckSelectEl.addEventListener("change", async () => {
      const id = Number(lobbyDeckSelectEl.value, 10);
      if (!id || !readStoredToken()) return;
      try {
        await fetch("/api/me/lobby-deck", {
          method: "PUT",
          headers: { "Content-Type": "application/json", ...authFetchHeaders() },
          body: JSON.stringify({ deckId: id }),
        });
      } catch (_) {
        /* ignore */
      }
    });
  }
  if (lobbyDeckViewBtnEl) lobbyDeckViewBtnEl.addEventListener("click", () => void openDeckViewModal());
  if (deckViewCloseBtnEl) deckViewCloseBtnEl.addEventListener("click", () => closeDeckViewModal());
  if (deckViewModalEl) {
    deckViewModalEl.addEventListener("click", (ev) => {
      if (ev.target === deckViewModalEl) closeDeckViewModal();
    });
  }
  if (promotionCancelEl) promotionCancelEl.addEventListener("click", () => closePromotionModal(null));
  if (promotionOverlayEl) {
    promotionOverlayEl.addEventListener("cancel", (ev) => {
      ev.preventDefault();
      closePromotionModal(null);
    });
  }
  if (deckSearchConfirmBtnEl) {
    deckSearchConfirmBtnEl.addEventListener("click", () => {
      if (!isGameplayInputOpen()) return;
      send("resolve_pending_effect", {});
      closeDeckSearchModal();
    });
  }
  document.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape" && promotionChoiceResolver) closePromotionModal(null);
  });
  if (authRegisterBtnEl) authRegisterBtnEl.addEventListener("click", () => void submitRegister());
  if (authLoginBtnEl) authLoginBtnEl.addEventListener("click", () => void submitLogin());
  if (logoutBtnEl) logoutBtnEl.addEventListener("click", () => logoutSession());
  if (localeSelectEl) localeSelectEl.addEventListener("change", () => setLocale(localeSelectEl.value));
  if (privateRoomEl) privateRoomEl.addEventListener("change", updatePrivatePasswordVisibility);
  if (roomPasswordEl) roomPasswordEl.addEventListener("input", () => hideLobbyPrivatePasswordError());
  if (roomPasswordToggleEl && roomPasswordEl) {
    roomPasswordToggleEl.addEventListener("click", () => {
      roomPasswordEl.type = roomPasswordEl.type === "password" ? "text" : "password";
      updatePasswordToggleVisual();
    });
  }
  if (roomNameEl) {
    roomNameEl.addEventListener("focus", () => roomNameEl.select());
    roomNameEl.addEventListener("pointerdown", () => {
      if (document.activeElement !== roomNameEl) setTimeout(() => roomNameEl.select(), 0);
    });
  }
  if (roomSearchEl) roomSearchEl.addEventListener("input", () => applyRoomSearch());

  function returnToLobbyAfterMatch() {
    hideMatchEndOverlay();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
  }

  // -----------------------------------------------------------------------
  // Match / playmat event listeners
  // -----------------------------------------------------------------------
  if (matchEndStayEl) {
    matchEndStayEl.addEventListener("click", () => {
      send("stay_in_room", {});
      hideMatchEndOverlay();
    });
  }
  if (matchEndRematchEl) {
    matchEndRematchEl.addEventListener("click", () => {
      send("request_rematch", {});
      matchEndRematchEl.disabled = true;
    });
  }
  if (matchEndToLobbyEl) matchEndToLobbyEl.addEventListener("click", () => returnToLobbyAfterMatch());
  if (matchEndOverlayEl) {
    matchEndOverlayEl.addEventListener("click", (ev) => {
      if (ev.target === matchEndOverlayEl) hideMatchEndOverlay();
    });
  }
  const disconnectBtnEl = document.getElementById("disconnectBtn");
  if (disconnectBtnEl) {
    disconnectBtnEl.addEventListener("click", () => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        send("leave_match", {});
        setTimeout(() => {
          if (ws && ws.readyState === WebSocket.OPEN) ws.close();
        }, 250);
      }
    });
  }
  if (exportClientTraceBtnEl) {
    exportClientTraceBtnEl.addEventListener("click", () => downloadClientTraceJson());
  }
  if (reactionModeSelectEl) {
    reactionModeSelectEl.addEventListener("change", () => {
      updateReactionModeLabel();
      if (joinedRoom) send("set_reaction_mode", { mode: reactionModeSelectEl.value });
    });
  }
  if (reactionPassBtnEl) {
    reactionPassBtnEl.addEventListener("click", () => {
      if (!isGameplayInputOpen()) return;
      if (!canPassReactionPriority(lastSnapshot, playerEl.value)) return;
      send("resolve_reactions", {});
    });
  }
  if (coordsInSquaresEl) {
    coordsInSquaresEl.addEventListener("change", () => {
      updateCoordsToggleLabel();
      if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
    });
  }
  if (effectTurnsAlwaysVisibleEl) {
    effectTurnsAlwaysVisibleEl.addEventListener("change", () => {
      updateEffectTurnsToggleLabel();
      if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
    });
  }
  if (playerEl) {
    playerEl.addEventListener("change", () => {
      syncPlayerRoleLabels();
      if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
    });
  }

  // -----------------------------------------------------------------------
  // Initialization
  // -----------------------------------------------------------------------
  let savedLocale = "en-US";
  try {
    savedLocale = localStorage.getItem("powerChessLocale") || "en-US";
  } catch (_) {
    savedLocale = "en-US";
  }
  setLocale(savedLocale);

  updatePrivatePasswordVisibility();
  updatePasswordToggleVisual();
  renderBoard([]);
  renderStatus({});
  void bootstrapAuthSession();
  startRoomListPolling();
  document.addEventListener("scroll", () => hideCardPreview(), true);
  window.addEventListener("resize", () => {
    hideCardPreview();
    renderCaptureThreatOverlay();
  });
})();
