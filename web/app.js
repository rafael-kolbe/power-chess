(function () {
  let ws = null;
  let seq = 1;
  let lastSnapshot = null;
  let selectedFrom = null;
  let highlightedMoves = [];
  let draggingFrom = null;
  let currentTurn = "A";
  let turnSeconds = 30;
  let turnDeadline = Date.now() + turnSeconds * 1000;
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
  const snapshotEl = document.getElementById("snapshot");
  const eventsEl = document.getElementById("events");
  const statusEl = document.getElementById("status");
  const playerEl = document.getElementById("playerId");
  const reactionToggleEl = document.getElementById("reactionToggle");
  const reactionToggleLabelEl = document.getElementById("reactionToggleLabel");
  const coordsInSquaresEl = document.getElementById("coordsInSquares");
  const clockAEl = document.getElementById("clockA");
  const clockBEl = document.getElementById("clockB");
  const manaFillA = document.getElementById("manaFillA");
  const manaFillB = document.getElementById("manaFillB");
  const manaLabelA = document.getElementById("manaLabelA");
  const manaLabelB = document.getElementById("manaLabelB");
  const energizedFillA = document.getElementById("energizedFillA");
  const energizedFillB = document.getElementById("energizedFillB");
  const energizedLabelA = document.getElementById("energizedLabelA");
  const energizedLabelB = document.getElementById("energizedLabelB");
  const strikesAEl = document.getElementById("strikesA");
  const strikesBEl = document.getElementById("strikesB");
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
      youLabel: "You",
      opponentLabel: "Opponent",
      passwordLabelInline: "Password",
      show: "show",
      hide: "hide",
      statusWaiting: "waiting",
      statusPlaying: "in game",
      reactions: "Reactions",
      toggleOn: "On",
      toggleOff: "Off",
      coordsInSquares: "Coords in squares",
      submitMove: "Submit move",
      activateCard: "Activate card",
      resolvePending: "Resolve pending effect",
      queueReaction: "Queue reaction",
      resolveReactions: "Resolve reactions",
      reactionPendingTitle: "Reaction/Pending status",
      snapshotTitle: "Snapshot",
      eventsTitle: "Events",
      clock: "Clock",
      strikes: "Strikes",
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
      reasonStrikeLimit: "Reason: defeat by 3 strikes (turn timeout).",
      youWon: "You won!",
      youLost: "You lost!",
      draw: "Draw!",
      matchEndedNoWinner: "Match ended.",
      reasonPrefix: "Reason",
      reasonOpponentAbandoned: "Reason: Your opponent abandoned the match.",
      reasonCheckmateShort: "Reason: checkmate.",
      reasonStalemateShort: "Reason: stalemate.",
      disconnectWinAlert: "Victory: opponent disconnected and did not return in time.",
      opponentDisconnectedTitle: "Opponent disconnected",
      opponentDisconnectedSeconds: "{s}s",
      opponentDisconnectedHint:
        "You will win when the timer reaches 0 if they do not return. This seat is closed to other players until then."
      ,
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
      debugLogsTitle: "Debug logs"
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
      waiting: "Aguardando adversário...",
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
      youLabel: "Você",
      opponentLabel: "Oponente",
      passwordLabelInline: "Senha",
      show: "mostrar",
      hide: "ocultar",
      statusWaiting: "aguardando",
      statusPlaying: "em partida",
      reactions: "Reações",
      toggleOn: "Ligado",
      toggleOff: "Desligado",
      coordsInSquares: "Coordenadas nas casas",
      submitMove: "Enviar jogada",
      activateCard: "Ativar carta",
      resolvePending: "Resolver efeito pendente",
      queueReaction: "Enfileirar reação",
      resolveReactions: "Resolver reações",
      reactionPendingTitle: "Status de reação/pendente",
      snapshotTitle: "Snapshot",
      eventsTitle: "Eventos",
      clock: "Relógio",
      strikes: "Strikes",
      fromPlaceholder: "origem linha,col",
      toPlaceholder: "destino linha,col",
      handIndexPlaceholder: "índice da mão",
      pendingPiecePlaceholder: "peça pendente linha,col",
      reactionHandIndexPlaceholder: "índice reação",
      reactionPiecePlaceholder: "alvo da reação linha,col (opcional)",
      reasonCheckmate: "Motivo: xeque-mate.",
      reasonStalemate: "Motivo: empate por afogamento.",
      reasonBothDisconnected: "Motivo: ambos desconectaram (partida cancelada).",
      reasonDisconnectTimeout: "Motivo: vitória por desconexão do adversário.",
      reasonLeftRoom: "Motivo: vitória por saída da sala do adversário.",
      reasonStrikeLimit: "Motivo: derrota por 3 strikes (tempo estourado).",
      youWon: "Você venceu!",
      youLost: "Você perdeu!",
      draw: "Empate!",
      matchEndedNoWinner: "Partida encerrada.",
      reasonPrefix: "Motivo",
      reasonOpponentAbandoned: "Motivo: Seu adversário abandonou a partida.",
      reasonCheckmateShort: "Motivo: Xeque-mate.",
      reasonStalemateShort: "Motivo: Afogamento (stalemate).",
      disconnectWinAlert: "Vitória: o adversário saiu da sala (tempo de reconexão expirou).",
      opponentDisconnectedTitle: "Oponente desconectou",
      opponentDisconnectedSeconds: "{s}s",
      opponentDisconnectedHint:
        "Você vence quando o tempo chegar a 0 se o adversário não voltar. Este lugar fica fechado para outros até lá."
      ,
      rematchProposed: "Novo jogo proposto, clique em 'Jogar novamente' para aceitar.",
      rematchWaiting: "Aguardando o adversário aceitar o novo jogo.",
      rematchOpponentLeft: "O outro jogador saiu da sala.",
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
      debugLogsTitle: "Logs de debug"
    }
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
    if (!msg) {
      authErrorEl.classList.add("hidden");
      authErrorEl.textContent = "";
      return;
    }
    authErrorEl.textContent = msg;
    authErrorEl.classList.remove("hidden");
  }

  function refreshLobbyUserLabel() {
    if (!authBackendAvailable) {
      lobbyUserLabelEl.textContent = t("lobbyGuest");
      logoutBtnEl.classList.add("hidden");
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
    if (!authBackendAvailable) return;
    authOverlayEl.classList.remove("hidden");
    authOverlayEl.setAttribute("aria-hidden", "false");
  }

  function hideAuthOverlay() {
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
      authUnavailableHintEl.classList.add("hidden");
      refreshLobbyUserLabel();
      return;
    }
    authBackendAvailable = true;
    if (r.ok) {
      authUser = await r.json();
      hideAuthOverlay();
      authUnavailableHintEl.classList.add("hidden");
      refreshLobbyUserLabel();
      return;
    }
    authUser = null;
    writeStoredToken("");
    if (r.status === 401) {
      showAuthOverlay();
      refreshLobbyUserLabel();
      return;
    }
    hideAuthOverlay();
    refreshLobbyUserLabel();
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
          confirm_password: confirm
        })
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
        body: JSON.stringify({ email, password })
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
    } catch (_) {
      setAuthErrorVisible(t("authErrorNetwork"));
    }
  }

  function logoutSession() {
    writeStoredToken("");
    authUser = null;
    if (authBackendAvailable) showAuthOverlay();
    refreshLobbyUserLabel();
    authLoginEmailEl.value = "";
    authLoginPasswordEl.value = "";
  }

  function applyTranslations() {
    document.getElementById("titleLabel").textContent = t("title");
    document.getElementById("languageLabel").textContent = t("language");
    document.getElementById("roomNameLabel").textContent = t("roomName");
    document.getElementById("pieceTypeLabel").textContent = t("pieceType");
    document.getElementById("privateRoomLabel").textContent = t("privateRoom");
    document.getElementById("passwordLabel").textContent = t("password");
    document.getElementById("lobbyHint").textContent = t("hint");
    document.getElementById("roomListTitle").textContent = t("openRooms");
    document.getElementById("roomSearchLabel").textContent = t("searchLabel");
    roomSearchEl.placeholder = t("searchPlaceholder");
    roomPasswordEl.placeholder = t("passwordPlaceholder");
    roomListEmptyEl.textContent = t("noRooms");
    waitingBannerEl.textContent = t("waiting");
    document.getElementById("matchEndTitle").textContent = t("matchFinished");
    matchEndRematchEl.textContent = t("playAgain");
    matchEndStayEl.textContent = t("stayInRoom");
    matchEndToLobbyEl.textContent = t("backLobby");
    document.getElementById("disconnectBtn").textContent = t("leaveRoom");
    document.getElementById("authTitle").textContent = t("authCreateTitle");
    document.getElementById("authUsernameLabel").textContent = t("authUsername");
    document.getElementById("authEmailLabel").textContent = t("authEmail");
    document.getElementById("authPasswordLabel").textContent = t("authPassword");
    document.getElementById("authConfirmPasswordLabel").textContent = t("authConfirmPassword");
    document.getElementById("authDividerLabel").textContent = t("authAlreadyHave");
    document.getElementById("authLoginEmailLabel").textContent = t("authEmail");
    document.getElementById("authLoginPasswordLabel").textContent = t("authPassword");
    authRegisterBtnEl.textContent = t("authRegister");
    authLoginBtnEl.textContent = t("authLogin");
    logoutBtnEl.textContent = t("authLogout");
    document.getElementById("reactionPendingTitle").textContent = t("reactionPendingTitle");
    document.getElementById("snapshotTitle").textContent = t("snapshotTitle");
    document.getElementById("eventsTitle").textContent = t("eventsTitle");
    const dbg = document.getElementById("debugLogsTitle");
    if (dbg) dbg.textContent = t("debugLogsTitle");
    document.getElementById("coordsInSquaresText").textContent = t("coordsInSquares");
    updateReactionToggleLabel();
    document.getElementById("clockLabelA").textContent = t("clock");
    document.getElementById("clockLabelB").textContent = t("clock");
    document.getElementById("strikesLabelA").textContent = t("strikes");
    document.getElementById("strikesLabelB").textContent = t("strikes");
    const optRandom = pieceTypeEl.querySelector('option[value="random"]');
    const optWhite = pieceTypeEl.querySelector('option[value="white"]');
    const optBlack = pieceTypeEl.querySelector('option[value="black"]');
    if (optRandom) optRandom.textContent = t("pieceTypeRandom");
    if (optWhite) optWhite.textContent = t("pieceTypeWhite");
    if (optBlack) optBlack.textContent = t("pieceTypeBlack");
    const connectBtn = document.getElementById("connectBtn");
    connectBtn.textContent = t("create");
    syncPlayerRoleLabels();
    refreshLobbyUserLabel();
    renderRoomList(lobbyRooms);
    if (lastSnapshot) {
      renderInRoomLabel(lastSnapshot);
    }
    if (joinedRoom && lastSnapshot) {
      updateOpponentDisconnectOverlay(lastSnapshot);
    }
    updatePasswordToggleVisual();
    refreshPrivateJoinModalTexts();
    if (!lobbyPrivatePasswordErrorEl.classList.contains("hidden")) {
      lobbyPrivatePasswordErrorEl.textContent = t("privateNeedsPassword");
    }
    if (cardMarqueeLabelEl) {
      cardMarqueeLabelEl.textContent = t("cardMarqueeTitle");
    }
  }

  /**
   * Updates copy on the private-room join modal if it is open (e.g. after locale change).
   */
  function refreshPrivateJoinModalTexts() {
    if (privateJoinOverlayEl.classList.contains("hidden")) return;
    privateJoinTitleEl.textContent = t("privateJoinTitle");
    privateJoinBodyEl.textContent = t("privateJoinDescription", { name: privateJoinPendingRoomName || "Let's Play!" });
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
        { signal }
      );
      privateJoinCancelEl.addEventListener("click", () => close(null), { signal });
      privateJoinSubmitEl.addEventListener("click", () => submit(), { signal });
      privateJoinPasswordInputEl.addEventListener(
        "input",
        () => {
          hidePrivateJoinError();
        },
        { signal }
      );
      privateJoinPasswordInputEl.addEventListener(
        "keydown",
        (e) => {
          if (e.key === "Enter") submit();
        },
        { signal }
      );
      privateJoinPasswordToggleEl.addEventListener(
        "click",
        () => {
          privateJoinPasswordInputEl.type = privateJoinPasswordInputEl.type === "password" ? "text" : "password";
          updatePrivateJoinToggleVisual();
        },
        { signal }
      );

      queueMicrotask(() => privateJoinPasswordInputEl.focus());
    });
  }

  function updateReactionToggleLabel() {
    if (!reactionToggleEl || !reactionToggleLabelEl) return;
    reactionToggleLabelEl.textContent = `${t("reactions")}: ${reactionToggleEl.checked ? t("toggleOn") : t("toggleOff")}`;
  }

  /**
   * @param {string} roleKey i18n key: youLabel or opponentLabel
   * @param {string} [name] server snapshot display name for that seat
   */
  function playerClockLabel(roleKey, name) {
    const role = t(roleKey);
    const n = name && String(name).trim();
    if (!n) return role;
    return `${role} (${n})`;
  }

  /**
   * @param {object} [snapshot] state_snapshot payload; falls back to lastSnapshot when omitted
   */
  function syncPlayerRoleLabels(snapshot) {
    const snap = snapshot !== undefined ? snapshot : lastSnapshot;
    const isA = playerEl.value === "A";
    const top = document.getElementById("playerBLabel");
    const bottom = document.getElementById("playerALabel");
    const nameA = snap?.playerAName ?? "";
    const nameB = snap?.playerBName ?? "";
    if (isA) {
      top.textContent = playerClockLabel("opponentLabel", nameB);
      bottom.textContent = playerClockLabel("youLabel", nameA);
    } else {
      top.textContent = playerClockLabel("youLabel", nameB);
      bottom.textContent = playerClockLabel("opponentLabel", nameA);
    }
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
    localeSelectEl.value = locale;
    try {
      localStorage.setItem("powerChessLocale", locale);
    } catch (_) {
      /* ignore storage failures */
    }
    applyTranslations();
    document.dispatchEvent(new CustomEvent("powerchess:locale", { detail: { locale } }));
  }

  function hideLobbyPrivatePasswordError() {
    lobbyPrivatePasswordErrorEl.textContent = "";
    lobbyPrivatePasswordErrorEl.classList.add("hidden");
  }

  function showLobbyPrivatePasswordError() {
    lobbyPrivatePasswordErrorEl.textContent = t("privateNeedsPassword");
    lobbyPrivatePasswordErrorEl.classList.remove("hidden");
  }

  function updatePrivatePasswordVisibility() {
    roomPasswordFieldEl.classList.toggle("hidden", !privateRoomEl.checked);
    if (!privateRoomEl.checked) {
      hideLobbyPrivatePasswordError();
    }
  }

  function updatePasswordToggleVisual() {
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

  function connectToRoom(roomId, pieceTypeOverride, roomNameOverride, privateOverride, passwordOverride) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
    joinedRoom = false;
    gameStarted = false;
    prevMatchEnded = false;
    hideMatchEndOverlay();
    hideLobbyPrivatePasswordError();
    const pieceType = pieceTypeOverride || pieceTypeEl.value || "random";
    const roomName = (roomNameOverride || roomNameEl.value || "Let's Play!").trim() || "Let's Play!";
    const creatingNewRoom = !String(roomId || "").trim();
    const isPrivate = typeof privateOverride === "boolean" ? privateOverride : (creatingNewRoom ? privateRoomEl.checked : false);
    let password = "";
    if (typeof passwordOverride === "string") {
      password = passwordOverride;
    } else if (creatingNewRoom) {
      password = roomPasswordEl.value;
    }
    if (isPrivate && !String(password || "").trim()) {
      showLobbyPrivatePasswordError();
      roomPasswordEl.focus();
      return;
    }
    playerEl.value = desiredPlayerForPieceType(pieceType);
    pendingJoinAttempt = {
      roomId: roomId || "",
      roomName,
      isPrivate,
      password,
      pieceType,
      attemptedFallback: false
    };
    syncPlayerRoleLabels();
    ws = new WebSocket(socketURLWithAuth());
    ws.onopen = () => {
      logEvent({ event: "socket_open" });
      send("join_match", {
        roomId: roomId || "",
        roomName,
        pieceType,
        playerId: playerEl.value,
        isPrivate,
        password
      });
    };
    ws.onclose = () => {
      logEvent({ event: "socket_close" });
      stopRoomListPolling();
      resetToLobbyUi();
    };
    ws.onerror = () => logEvent({ event: "socket_error" });
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data);
      logEvent(msg);
      if (msg.type === "state_snapshot") {
        pendingJoinAttempt = null;
        lastSnapshot = msg.payload;
        selectedFrom = null;
        highlightedMoves = [];

        if (!joinedRoom) {
          joinedRoom = true;
          setLobbyFooterVisible(false);
          lobbyScreenEl.classList.add("hidden");
          gameShellEl.classList.remove("hidden");
          playerEl.disabled = true;
          roomNameEl.disabled = true;
          roomSearchEl.disabled = true;
          privateRoomEl.disabled = true;
          roomPasswordEl.disabled = true;
          pieceTypeEl.disabled = true;
          stopRoomListPolling();
          syncBoardPerspectiveClass();
        }

        const wasStarted = gameStarted;
        updateLobbyChromeFromSnapshot(msg.payload);
        if (msg.payload.gameStarted && !wasStarted) {
          turnSeconds = turnSecondsFromSnapshot(msg.payload);
          currentTurn = msg.payload.turnPlayer || "A";
          turnDeadline = Date.now() + turnSeconds * 1000;
        }
        syncTurnFromSnapshot(msg.payload);
        updateOpponentDisconnectOverlay(msg.payload);
        maybeShowMatchEndModal(msg.payload);

        snapshotEl.textContent = JSON.stringify(msg.payload, null, 2);
        renderBoard(msg.payload.board);
        renderStatus(msg.payload);
        renderPlayerHud(msg.payload);
        syncPlayerRoleLabels(msg.payload);
        handleAutoSkipReaction(msg.payload);
        renderTurnClocks();
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
          password: pendingJoinAttempt.password
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
    P: "Pawn"
  };

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
    const knightJumps = [[-2, -1], [-2, 1], [-1, -2], [-1, 2], [1, -2], [1, 2], [2, -1], [2, 1]];
    const kingSteps = [[-1, -1], [-1, 0], [-1, 1], [0, -1], [0, 1], [1, -1], [1, 0], [1, 1]];

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

  /**
   * computeMoves returns pseudo-legal destinations for highlighting. En passant uses server snapshot.
   * @param {string[][]} board
   * @param {{row:number,col:number}} from logical coords
   * @param {{valid?:boolean,targetRow?:number,targetCol?:number,pawnRow?:number,pawnCol?:number}} [ep]
   * @param {{whiteKingSide?:boolean,whiteQueenSide?:boolean,blackKingSide?:boolean,blackQueenSide?:boolean}} [castlingRights]
   * @returns {{row:number,col:number}[]}
   */
  function computeMoves(board, from, ep, castlingRights) {
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
        if (
          ep &&
          ep.valid &&
          rr === ep.targetRow &&
          cc === ep.targetCol
        ) {
          const cap = parseCode(pieceAt(board, ep.pawnRow, ep.pawnCol));
          if (cap && cap.type === "P" && cap.color !== color) {
            out.push({ row: rr, col: cc });
          }
        }
      }
      return out.filter((m) => inBounds(m.row, m.col));
    }

    if (type === "N") {
      const jumps = [[-2, -1], [-2, 1], [-1, -2], [-1, 2], [1, -2], [1, 2], [2, -1], [2, 1]];
      for (const [dr, dc] of jumps) pushIfValidMove(out, board, color, from.row + dr, from.col + dc);
      return out;
    }
    if (type === "B") {
      slidingMoves(out, board, color, from, [[-1, -1], [-1, 1], [1, -1], [1, 1]]);
      return out;
    }
    if (type === "R") {
      slidingMoves(out, board, color, from, [[-1, 0], [1, 0], [0, -1], [0, 1]]);
      return out;
    }
    if (type === "Q") {
      slidingMoves(out, board, color, from, [[-1, -1], [-1, 1], [1, -1], [1, 1], [-1, 0], [1, 0], [0, -1], [0, 1]]);
      return out;
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
        if (kingSideRight && rookKingSide && rookKingSide.type === "R" && rookKingSide.color === color && emptyF && emptyG && safeE && safeF && safeG) {
          out.push({ row: homeRow, col: 6 });
        }
        if (queenSideRight && rookQueenSide && rookQueenSide.type === "R" && rookQueenSide.color === color && emptyB && emptyC && emptyD && safeE && safeD && safeC) {
          out.push({ row: homeRow, col: 2 });
        }
      }
    }
    return out;
  }

  function isOwnPiece(code) {
    const p = parseCode(code);
    if (!p) return false;
    const local = playerEl.value === "A" ? "w" : "b";
    return p.color === local;
  }

  function setBar(fillEl, labelEl, cur, max) {
    const m = Math.max(1, max || 1);
    const pct = Math.min(100, Math.round((100 * (cur || 0)) / m));
    fillEl.style.width = `${pct}%`;
    labelEl.textContent = `${cur ?? 0}/${max ?? 0}`;
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
        strikesAEl.textContent = String(p.strikes ?? 0);
        setBar(manaFillA, manaLabelA, p.mana, p.maxMana);
        setBar(energizedFillA, energizedLabelA, p.energizedMana, p.maxEnergized);
      } else if (p.playerId === "B") {
        strikesBEl.textContent = String(p.strikes ?? 0);
        setBar(manaFillB, manaLabelB, p.mana, p.maxMana);
        setBar(energizedFillB, energizedLabelB, p.energizedMana, p.maxEnergized);
      }
    }
  }

  function clocksActive(snapshot) {
    return snapshot && snapshot.gameStarted === true && !snapshot.matchEnded;
  }

  function renderTurnClocks() {
    const snap = lastSnapshot;
    if (!clocksActive(snap)) {
      clockAEl.textContent = "--";
      clockBEl.textContent = "--";
      return;
    }
    const secLeft = Math.max(0, Math.ceil((turnDeadline - Date.now()) / 1000));
    if (currentTurn === "A") {
      clockAEl.textContent = String(secLeft);
      clockBEl.textContent = String(turnSeconds);
    } else {
      clockAEl.textContent = String(turnSeconds);
      clockBEl.textContent = String(secLeft);
    }
  }

  function turnSecondsFromSnapshot(payload) {
    const v = Number(payload?.turnSeconds);
    if (!Number.isFinite(v) || v <= 0) return 30;
    return Math.round(v);
  }

  function handleAutoSkipReaction(snapshot) {
    if (reactionToggleEl.checked) return;
    const rw = snapshot?.reactionWindow;
    if (!rw?.open) return;
    const localPlayer = playerEl.value;
    if (rw.stackSize === 0 && rw.actor && rw.actor !== localPlayer) {
      send("resolve_reactions", {});
    }
  }

  function sendMove(from, to) {
    send("submit_move", {
      fromRow: from.row, fromCol: from.col,
      toRow: to.row, toCol: to.col
    });
  }

  function logEvent(obj) {
    const line = JSON.stringify(obj);
    eventsEl.textContent = `${line}\n${eventsEl.textContent}`.slice(0, 8000);
  }

  function send(type, payload) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ id: `req-${seq++}`, type, payload }));
  }

  function makeEdgeLabel(text) {
    const el = document.createElement("div");
    el.className = "edge-label";
    el.textContent = text;
    return el;
  }

  function renderBoard(board) {
    syncBoardPerspectiveClass();
    boardFrameEl.innerHTML = "";
    boardFrameEl.classList.toggle("show-inner-coords", coordsInSquaresEl && coordsInSquaresEl.checked);
    const moveSet = new Set(highlightedMoves.map((m) => posKey(m.row, m.col)));
    const selectedKey = selectedFrom ? posKey(selectedFrom.row, selectedFrom.col) : null;
    const ep = lastSnapshot?.enPassant;

    for (let gr = 0; gr < 10; gr++) {
      for (let gc = 0; gc < 10; gc++) {
        const corner =
          (gr === 0 || gr === 9) && (gc === 0 || gc === 9);
        if (corner) {
          const cornerEl = document.createElement("div");
          cornerEl.className = "edge-label";
          boardFrameEl.appendChild(cornerEl);
          continue;
        }

        if (gr === 0 && gc >= 1 && gc <= 8) {
          const dc = gc - 1;
          boardFrameEl.appendChild(makeEdgeLabel(fileLetterFromDisplayEdge(0, dc)));
          continue;
        }
        if (gr === 9 && gc >= 1 && gc <= 8) {
          const dc = gc - 1;
          boardFrameEl.appendChild(makeEdgeLabel(fileLetterFromDisplayEdge(7, dc)));
          continue;
        }
        if (gc === 0 && gr >= 1 && gr <= 8) {
          const dr = gr - 1;
          boardFrameEl.appendChild(makeEdgeLabel(rankDigitFromDisplayEdge(dr, 0)));
          continue;
        }
        if (gc === 9 && gr >= 1 && gr <= 8) {
          const dr = gr - 1;
          boardFrameEl.appendChild(makeEdgeLabel(rankDigitFromDisplayEdge(dr, 7)));
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
        sq.title = code ? `${code} ${logicalToAlgebraic(r, c)}` : logicalToAlgebraic(r, c);
        sq.dataset.row = String(r);
        sq.dataset.col = String(c);
        sq.dataset.code = code;
        sq.draggable = !!code;

        sq.addEventListener("click", () => {
          if (!lastSnapshot?.board || !gameStarted) return;
          const clickedCode = sq.dataset.code || "";
          if (clickedCode && isOwnPiece(clickedCode)) {
            selectedFrom = logical;
            highlightedMoves = computeMoves(lastSnapshot.board, selectedFrom, ep, lastSnapshot?.castlingRights);
            renderBoard(lastSnapshot.board);
            return;
          }
          if (selectedFrom && moveSet.has(posKey(r, c))) {
            sendMove(selectedFrom, logical);
            selectedFrom = null;
            highlightedMoves = [];
            renderBoard(lastSnapshot.board);
          }
        });

        sq.addEventListener("dragstart", (ev) => {
          const dragCode = sq.dataset.code || "";
          if (!gameStarted || !dragCode || !isOwnPiece(dragCode)) {
            ev.preventDefault();
            return;
          }
          draggingFrom = logical;
          selectedFrom = logical;
          highlightedMoves = computeMoves(lastSnapshot?.board, draggingFrom, lastSnapshot?.enPassant, lastSnapshot?.castlingRights);
          renderBoard(lastSnapshot?.board || []);
          ev.dataTransfer?.setData("text/plain", posKey(r, c));
          ev.dataTransfer.effectAllowed = "move";
        });

        sq.addEventListener("dragover", (ev) => {
          if (!draggingFrom) return;
          if (!moveSet.has(posKey(r, c))) return;
          ev.preventDefault();
          sq.classList.add("drop-target");
          ev.dataTransfer.dropEffect = "move";
        });

        sq.addEventListener("dragleave", () => {
          sq.classList.remove("drop-target");
        });

        sq.addEventListener("drop", (ev) => {
          sq.classList.remove("drop-target");
          if (!draggingFrom || !moveSet.has(posKey(r, c))) return;
          ev.preventDefault();
          const from = draggingFrom;
          const to = logical;
          draggingFrom = null;
          selectedFrom = null;
          highlightedMoves = [];
          sendMove(from, to);
        });

        sq.addEventListener("dragend", () => {
          draggingFrom = null;
          selectedFrom = null;
          highlightedMoves = [];
          if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
        });

        boardFrameEl.appendChild(sq);
      }
    }
  }

  function renderStatus(snapshot) {
    const rw = snapshot?.reactionWindow || {};
    const pc = snapshot?.pendingCapture || {};
    statusEl.textContent = JSON.stringify(
      {
        pendingCapture: pc,
        reactionWindow: rw,
        pendingEffects: snapshot?.pendingEffects || []
      },
      null,
      2
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
    const show =
      payload &&
      !payload.matchEnded &&
      pending &&
      pending !== local &&
      deadline > 0;
    if (!show) {
      hideOpponentDisconnectOverlay();
      return;
    }
    opponentDisconnectOverlayEl.classList.remove("hidden");
    opponentDisconnectOverlayEl.setAttribute("aria-hidden", "false");
    document.getElementById("opponentDisconnectTitle").textContent = t("opponentDisconnectedTitle");
    document.getElementById("opponentDisconnectHint").textContent = t("opponentDisconnectedHint");
    const countdownEl = document.getElementById("opponentDisconnectCountdown");
    const tick = () => {
      const sec = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      countdownEl.textContent = t("opponentDisconnectedSeconds", { s: sec });
      if (sec <= 0) {
        hideOpponentDisconnectOverlay();
      }
    };
    tick();
    clearOpponentDisconnectInterval();
    opponentDisconnectInterval = setInterval(tick, 500);
  }

  function hideMatchEndOverlay() {
    matchEndOverlayEl.classList.add("hidden");
    matchEndOverlayEl.setAttribute("aria-hidden", "true");
    matchEndRematchEl.classList.add("hidden");
    matchEndRematchEl.disabled = false;
    matchEndStayEl.classList.add("hidden");
    matchEndCountdownEl.classList.add("hidden");
    matchEndCountdownEl.textContent = "";
    if (matchCountdownTimer) {
      clearInterval(matchCountdownTimer);
      matchCountdownTimer = null;
    }
  }

  function showMatchEndOverlay(title, bodyText) {
    document.getElementById("matchEndTitle").textContent = title;
    matchEndBodyEl.textContent = bodyText;
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
      strike_limit: t("reasonStrikeLimit")
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
    } else if (!matchEndOverlayEl.classList.contains("hidden")) {
      matchEndBodyEl.textContent = buildPostMatchModalMessage(payload);
    }
    prevMatchEnded = true;
  }

  function updatePostMatchActionControls(payload) {
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
      const next = { ...lastSnapshot, postMatchMsLeft: Math.max(0, Number(lastSnapshot.postMatchMsLeft || 0) - 1000) };
      lastSnapshot = next;
      tick();
    }, 1000);
  }

  function updateLobbyChromeFromSnapshot(payload) {
    const started = payload.gameStarted === true;
    const ended = payload.matchEnded === true;
    gameStarted = started;
    waitingBannerEl.classList.toggle("hidden", started || ended);
    boardAreaEl.classList.remove("hidden");
    renderInRoomLabel(payload);
  }

  function renderInRoomLabel(payload) {
    const roomName = payload.roomName || "Let's Play!";
    const roomId = payload.roomId || "";
    const privacy = payload.roomPrivate ? t("private") : t("public");
    inRoomLabelEl.textContent = "";
    inRoomLabelEl.append(
      `${t("room")}: ${roomName} (#${roomId}) | ${privacy}`
    );
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
    const youRoom =
      authUser && authUser.username
        ? `${t("you")}: ${authUser.username} (${t("player")} ${playerEl.value})`
        : `${t("you")}: ${t("player")} ${playerEl.value}`;
    inRoomLabelEl.append(` | ${youRoom}`);
  }

  function syncTurnFromSnapshot(payload) {
    if (!clocksActive(payload)) return;
    turnSeconds = turnSecondsFromSnapshot(payload);
    if (payload.turnPlayer && payload.turnPlayer !== currentTurn) {
      currentTurn = payload.turnPlayer;
      turnDeadline = Date.now() + turnSeconds * 1000;
    }
  }

  function resetToLobbyUi() {
    joinedRoom = false;
    gameStarted = false;
    lastSnapshot = null;
    setLobbyFooterVisible(true);
    lobbyScreenEl.classList.remove("hidden");
    gameShellEl.classList.add("hidden");
    playerEl.disabled = false;
    roomNameEl.disabled = false;
    roomSearchEl.disabled = false;
    privateRoomEl.disabled = false;
    roomPasswordEl.disabled = false;
    pieceTypeEl.disabled = false;
    waitingBannerEl.classList.add("hidden");
    boardAreaEl.classList.remove("hidden");
    renderBoard([]);
    renderStatus({});
    snapshotEl.textContent = "";
    inRoomLabelEl.textContent = "";
    currentTurn = "A";
    turnSeconds = 30;
    turnDeadline = Date.now() + turnSeconds * 1000;
    revealRoomPassword = false;
    prevMatchEnded = false;
    hideMatchEndOverlay();
    hideOpponentDisconnectOverlay();
    hideLobbyPrivatePasswordError();
    renderTurnClocks();
    startRoomListPolling();
  }

  function renderRoomList(rooms) {
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
      const lock = rm.roomPrivate ? '<img class="room-lock-icon" src="/public/lock-keyhole.png" alt="Sala privada" title="Sala privada">' : "";
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
          connectToRoom(rm.roomId, pieceType, rm.roomName || "Let's Play!", false, joinPassword);
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
    const filtered = lobbyRooms.filter((rm) =>
      String(rm.roomId || "").toLowerCase().includes(q) ||
      String(rm.roomName || "").toLowerCase().includes(q)
    );
    renderRoomList(filtered);
  }

  function pieceTypeForRoomJoin(rm) {
    if ((rm.connectedA || 0) > 0 && (rm.connectedB || 0) === 0) return "black";
    if ((rm.connectedB || 0) > 0 && (rm.connectedA || 0) === 0) return "white";
    return pieceTypeEl.value || "random";
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

  document.getElementById("connectBtn").addEventListener("click", () => {
    connectToRoom("", pieceTypeEl.value, roomNameEl.value);
  });

  function returnToLobbyAfterMatch() {
    hideMatchEndOverlay();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
  }

  matchEndStayEl.addEventListener("click", () => {
    send("stay_in_room", {});
    hideMatchEndOverlay();
  });
  matchEndRematchEl.addEventListener("click", () => {
    send("request_rematch", {});
    matchEndRematchEl.disabled = true;
  });
  matchEndToLobbyEl.addEventListener("click", () => returnToLobbyAfterMatch());
  matchEndOverlayEl.addEventListener("click", (ev) => {
    if (ev.target === matchEndOverlayEl) hideMatchEndOverlay();
  });

  document.getElementById("disconnectBtn").addEventListener("click", () => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      send("leave_match", {});
      setTimeout(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.close();
        }
      }, 250);
    }
  });

  authRegisterBtnEl.addEventListener("click", () => {
    void submitRegister();
  });
  authLoginBtnEl.addEventListener("click", () => {
    void submitLogin();
  });
  logoutBtnEl.addEventListener("click", () => logoutSession());

  localeSelectEl.addEventListener("change", () => setLocale(localeSelectEl.value));
  privateRoomEl.addEventListener("change", updatePrivatePasswordVisibility);
  roomPasswordEl.addEventListener("input", () => {
    hideLobbyPrivatePasswordError();
  });
  roomPasswordToggleEl.addEventListener("click", () => {
    roomPasswordEl.type = roomPasswordEl.type === "password" ? "text" : "password";
    updatePasswordToggleVisual();
  });
  roomNameEl.addEventListener("focus", () => roomNameEl.select());
  roomNameEl.addEventListener("pointerdown", () => {
    if (document.activeElement !== roomNameEl) {
      setTimeout(() => roomNameEl.select(), 0);
    }
  });
  roomSearchEl.addEventListener("input", () => applyRoomSearch());
  reactionToggleEl.addEventListener("change", updateReactionToggleLabel);
  coordsInSquaresEl.addEventListener("change", () => {
    if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
  });
  playerEl.addEventListener("change", () => {
    syncPlayerRoleLabels();
    if (lastSnapshot?.board) renderBoard(lastSnapshot.board);
  });
  globalThis.setInterval(renderTurnClocks, 250);
  renderTurnClocks();
  renderBoard([]);
  renderStatus({});
  updatePrivatePasswordVisibility();
  updatePasswordToggleVisual();
  let savedLocale = "en-US";
  try {
    savedLocale = localStorage.getItem("powerChessLocale") || "en-US";
  } catch (_) {
    savedLocale = "en-US";
  }
  setLocale(savedLocale);
  void bootstrapAuthSession();
  startRoomListPolling();
})();
