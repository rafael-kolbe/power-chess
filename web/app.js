(function () {
    /**
     * When `true`, the match playmat renders fake data (5 hand cards, 3 banished, 6 cooldown per side)
     * for layout testing. Does not touch the server. Set to `true` only while tuning CSS.
     */
    const PLAYMAT_UI_TEST_OVERLAY = false;

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
    /** @type {Set<number>} Hand indices selected to return to the deck during mulligan (local UI only). */
    let mulliganPick = new Set();

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
    const coordsInSquaresTextEl = document.getElementById("coordsInSquaresText");
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
            coordsLabel: "Coords",
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
            noSavedDeckAlert:
                "You have no saved deck. Use Deck Builder to create one (20 cards) before playing.",
            lobbyDeckView: "View",
            lobbyDeckBuilder: "Deck Builder",
            deckViewClose: "Close",
            debugLogsTitle: "Debug logs",
            zoneHand: "Hand",
            zoneDeck: "Deck",
            drawFromDeck: "DRAW",
            connectErrorPrefix: "Could not connect:",
            mulliganHint: "Tap cards to mark them red (they return to the deck). Confirm when ready.",
            mulliganConfirm: "Confirm mulligan",
            mulliganWaitingYou: "Waiting for you to confirm…",
            mulliganWaitingOpp: "Waiting for opponent…",
            mulliganLine: "Mulligan — White: {w} | Black: {b}",
            mulliganPending: "…",
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
            coordsLabel: "Coords",
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
                "Você vence quando o tempo chegar a 0 se o adversário não voltar. Este lugar fica fechado para outros até lá.",
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
            drawFromDeck: "Comprar",
            connectErrorPrefix: "Não foi possível conectar:",
            mulliganHint: "Toque nas cartas para marcar em vermelho (voltam ao deck). Confirme quando terminar.",
            mulliganConfirm: "Confirmar mulligan",
            mulliganWaitingYou: "Aguardando sua confirmação…",
            mulliganWaitingOpp: "Aguardando o oponente…",
            mulliganLine: "Mulligan — Brancas: {w} | Pretas: {b}",
            mulliganPending: "…",
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
                const msg =
                    r.status === 401 ? t("authErrorInvalid") : await authResponseErrorMessage(r, "authErrorGeneric");
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
        if (pmEl.drawBtn) pmEl.drawBtn.textContent = t("drawFromDeck");
        if (lastSnapshot) renderMulliganBar(lastSnapshot);
        updateCoordsToggleLabel();
        updateReactionToggleLabel();
        s("clockLabelA", "clock");
        s("clockLabelB", "clock");
        s("strikesLabelA", "strikes");
        s("strikesLabelB", "strikes");
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
                    privateJoinPasswordInputEl.type =
                        privateJoinPasswordInputEl.type === "password" ? "text" : "password";
                    updatePrivateJoinToggleVisual();
                },
                { signal },
            );

            queueMicrotask(() => privateJoinPasswordInputEl.focus());
        });
    }

    function updateReactionToggleLabel() {
        if (!reactionToggleEl || !reactionToggleLabelEl) return;
        reactionToggleLabelEl.textContent = `${t("reactions")}: ${reactionToggleEl.checked ? t("toggleOn") : t("toggleOff")}`;
    }

    function updateCoordsToggleLabel() {
        if (!coordsInSquaresEl || !coordsInSquaresTextEl) return;
        coordsInSquaresTextEl.textContent = `${t("coordsLabel")}: ${coordsInSquaresEl.checked ? t("toggleOn") : t("toggleOff")}`;
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
        if (!playerEl) return;
        const isA = playerEl.value === "A";
        const top = document.getElementById("playerBLabel");
        const bottom = document.getElementById("playerALabel");
        if (!top || !bottom) return;
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
            (roomNameOverride || (roomNameEl ? roomNameEl.value : "Let's Play!") || "Let's Play!").trim() ||
            "Let's Play!";
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

                if (snapshotEl) snapshotEl.textContent = JSON.stringify(msg.payload, null, 2);
                renderBoard(msg.payload.board);
                renderStatus(msg.payload);
                renderPlayerHud(msg.payload);
                runSnapshotAnimations(pmPrevSnapshot, msg.payload);
                renderPlaymat(msg.payload);
                pmPrevSnapshot = msg.payload;
                syncPlayerRoleLabels(msg.payload);
                handleAutoSkipReaction(msg.payload);
                renderTurnClocks();
                return;
            }
            if (
                isJoinOccupiedSideError(msg) &&
                !joinedRoom &&
                pendingJoinAttempt &&
                !pendingJoinAttempt.attemptedFallback
            ) {
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
                if (ep && ep.valid && rr === ep.targetRow && cc === ep.targetCol) {
                    const cap = parseCode(pieceAt(board, ep.pawnRow, ep.pawnCol));
                    if (cap && cap.type === "P" && cap.color !== color) {
                        out.push({ row: rr, col: cc });
                    }
                }
            }
            return out.filter((m) => inBounds(m.row, m.col));
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
            return out;
        }
        if (type === "B") {
            slidingMoves(out, board, color, from, [
                [-1, -1],
                [-1, 1],
                [1, -1],
                [1, 1],
            ]);
            return out;
        }
        if (type === "R") {
            slidingMoves(out, board, color, from, [
                [-1, 0],
                [1, 0],
                [0, -1],
                [0, 1],
            ]);
            return out;
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
                const queenSideRight =
                    color === "w" ? !!castlingRights?.whiteQueenSide : !!castlingRights?.blackQueenSide;
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
        return out;
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
        pileViewModal: document.getElementById("pileViewModal"),
        pileViewGrid: document.getElementById("pileViewGrid"),
        pileViewTitle: document.getElementById("pileViewTitle"),
        pileViewCloseBtn: document.getElementById("pileViewCloseBtn"),
        banishSelf: document.getElementById("banishSelf"),
        banishOpp: document.getElementById("banishOpp"),
        cooldownSelf: document.getElementById("cooldownSelf"),
        cooldownOpp: document.getElementById("cooldownOpp"),
        ignitionSelf: document.getElementById("ignitionSelf"),
        ignitionOpp: document.getElementById("ignitionOpp"),
        mulliganBar: document.getElementById("mulliganBar"),
        mulliganHint: document.getElementById("mulliganHint"),
        mulliganCounts: document.getElementById("mulliganCounts"),
        mulliganConfirmBtn: document.getElementById("mulliganConfirmBtn"),
    };

    // ---------------------------------------------------------------------------
    // Playmat: card preview hover (hover shows full card at cursor)
    // ---------------------------------------------------------------------------
    let pmPreviewCard = null;
    /** @type {string | null} Catalog id for the card currently shown in the match hover preview. */
    let pmPreviewCardId = null;
    /** Remember description vs example per catalog id while on the match screen (same idea as deck builder). */
    const matchHandExampleMode = new Map();

    /**
     * Syncs the floating match preview when the user toggles example/description on a hand card.
     * @param {string} cardId
     * @param {boolean} showingExample
     */
    function syncMatchPreviewExample(cardId, showingExample) {
        if (pmPreviewCardId !== cardId) return;
        const article = pmEl.matchCardPreview?.querySelector(".power-card");
        if (article && typeof globalThis.setPowerCardExampleMode === "function") {
            globalThis.setPowerCardExampleMode(article, showingExample);
        }
    }

    /**
     * Syncs all in-hand miniatures for a catalog id when the user toggles on the large hover card.
     * @param {string} cardId
     * @param {boolean} showingExample
     */
    function syncMatchHandExample(cardId, showingExample) {
        const root = document.getElementById("handSelf");
        if (!root || !cardId) return;
        root.querySelectorAll(".pm-hand-card-wrap").forEach((wrap) => {
            if (wrap.dataset.cardId !== cardId) return;
            const article = wrap.querySelector(".power-card");
            if (article && typeof globalThis.setPowerCardExampleMode === "function") {
                globalThis.setPowerCardExampleMode(article, showingExample);
            }
        });
    }

    function showCardPreview(cardData, anchorEl) {
        if (!pmEl.matchCardPreview || !cardData) return;
        pmEl.matchCardPreview.innerHTML = "";
        const cid = cardData.id != null ? String(cardData.id) : "";
        pmPreviewCardId = cid || null;
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
                syncMatchHandExample(cid, showing);
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
        pmPreviewCard = null;
        pmPreviewCardId = null;
    }

    function getCardDef(cardId) {
        const catalog = getLocalizedCardCatalog(locale);
        return catalog.find((c) => c.id === cardId) || null;
    }

    /**
     * @returns {boolean} Whether {@link applyPlaymatUiTestOverlay} should run (see {@link PLAYMAT_UI_TEST_OVERLAY}).
     */
    function isPlaymatUiTestEnabled() {
        return (
            PLAYMAT_UI_TEST_OVERLAY &&
            gameShellEl &&
            !gameShellEl.classList.contains("hidden")
        );
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
     * Flies a visual clone of `cardEl` from `fromRect` to `toRect`, then calls `done`.
     * Used for all "fluid movement" animations between zones.
     * @param {DOMRect} fromRect - source bounding rect
     * @param {DOMRect} toRect   - destination bounding rect
     * @param {HTMLElement|null} cardEl - element to clone (null = use sleeve)
     * @param {string} [sleeve="blue"] - sleeve color when no card face available
     * @param {number} [duration=400]  - animation duration in ms
     * @param {Function} [done]        - called when animation completes
     */
    function flyCard(fromRect, toRect, cardEl, sleeve, duration, done) {
        duration = duration || 400;
        const overlay = document.createElement("div");
        overlay.style.cssText = [
            "position:fixed",
            "pointer-events:none",
            `width:${fromRect.width}px`,
            `height:${fromRect.height}px`,
            `left:${fromRect.left}px`,
            `top:${fromRect.top}px`,
            "z-index:3000",
            "border-radius:4px",
            "box-shadow:0 8px 24px rgba(0,0,0,.7)",
            "transform-origin:center center",
        ].join(";");

        if (cardEl) {
            const clone = cardEl.cloneNode(true);
            clone.style.cssText = "width:100%;height:100%;pointer-events:none";
            overlay.appendChild(clone);
        } else {
            overlay.style.background = `url('${sleeveUrl(sleeve)}') center/100% 100% no-repeat #263040`;
        }

        document.body.appendChild(overlay);

        const dx = toRect.left + toRect.width / 2 - (fromRect.left + fromRect.width / 2);
        const dy = toRect.top + toRect.height / 2 - (fromRect.top + fromRect.height / 2);
        const scaleX = toRect.width / fromRect.width;
        const scaleY = toRect.height / fromRect.height;

        const anim = overlay.animate(
            [
                { transform: "translate(0,0) scale(1)", opacity: 1 },
                { transform: `translate(${dx}px,${dy}px) scale(${scaleX},${scaleY})`, opacity: 0.9 },
            ],
            { duration, easing: "cubic-bezier(0.22,0.61,0.36,1)", fill: "forwards" },
        );

        anim.onfinish = () => {
            overlay.remove();
            if (done) done();
        };
    }

    /** Returns the bounding rect of the given element or null. */
    function zoneRect(el) {
        return el ? el.getBoundingClientRect() : null;
    }

    /**
     * Animates counter value change on a given element.
     * Briefly highlights (pulse) when the number decreases.
     */
    function animateCounter(el, oldVal, newVal) {
        if (!el || oldVal === newVal) return;
        el.textContent = String(newVal);
        if (newVal < oldVal) {
            el.animate(
                [
                    { color: "#fff", textShadow: "0 0 12px #f0c040", transform: "scale(1.4)" },
                    { color: "#ffdc80", textShadow: "0 0 6px #000", transform: "scale(1)" },
                ],
                { duration: 500, easing: "ease-out" },
            );
        }
    }

    /**
     * Runs all animations needed when transitioning from prevSnap to nextSnap.
     * Animations are fire-and-forget; the DOM update happens normally after.
     */
    function runSnapshotAnimations(prevSnap, nextSnap) {
        if (!prevSnap || !nextSnap) return;
        const localPID = playerEl.value;

        const prevSelf = prevSnap.players?.find((p) => p.playerId === localPID);
        const nextSelf = nextSnap.players?.find((p) => p.playerId === localPID);
        const prevOpp = prevSnap.players?.find((p) => p.playerId !== localPID);
        const nextOpp = nextSnap.players?.find((p) => p.playerId !== localPID);

        if (!prevSelf || !nextSelf) return;

        // 1. Card draw: self hand count increased → fly from deck to last hand slot.
        if (nextSelf.handCount > prevSelf.handCount) {
            animateCardDraw(pmEl.deckSelf, pmEl.handSelf, false, nextSelf.sleeveColor);
        }
        // Opponent drew a card (always show sleeve, never reveal face).
        if (nextOpp && prevOpp && nextOpp.handCount > prevOpp.handCount) {
            animateCardDraw(pmEl.deckOpp, pmEl.handOpp, true, nextOpp.sleeveColor);
        }

        // 2. Ignition counter decrease (card was in ignition both before and after, turns went down).
        if (
            prevSnap.ignitionOn &&
            nextSnap.ignitionOn &&
            prevSnap.ignitionOwner === nextSnap.ignitionOwner &&
            nextSnap.ignitionTurnsRemaining < prevSnap.ignitionTurnsRemaining
        ) {
            const counterEl = nextSnap.ignitionOwner === localPID ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;
            animateCounter(counterEl, prevSnap.ignitionTurnsRemaining, nextSnap.ignitionTurnsRemaining);
        }

        // 3a. Card placed in ignition: was not occupied, now is → brief glow on ignition zone.
        if (!prevSnap.ignitionOn && nextSnap.ignitionOn) {
            const slotEl = nextSnap.ignitionOwner === localPID ? pmEl.ignitionSelf : pmEl.ignitionOpp;
            if (slotEl) {
                slotEl.classList.remove("pm-ignition-activating");
                void slotEl.offsetWidth; // force reflow to restart animation
                slotEl.classList.add("pm-ignition-activating");
                setTimeout(() => slotEl.classList.remove("pm-ignition-activating"), 650);
            }
        }

        // 3b. Ignition resolved (was occupied, now gone) → fly to cooldown pile.
        if (prevSnap.ignitionOn && !nextSnap.ignitionOn && prevSnap.ignitionOwner) {
            const wasOwn = prevSnap.ignitionOwner === localPID;
            const fromEl = wasOwn ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
            const toEl = wasOwn ? pmEl.cooldownCardsSelf : pmEl.cooldownCardsOpp;
            const fr = zoneRect(fromEl);
            const tr = zoneRect(toEl);
            if (fr && tr) {
                const def = getCardDef(prevSnap.ignitionCard);
                const face = def
                    ? (() => {
                          const el = createPowerCard({
                              type: def.type,
                              name: def.name,
                              description: def.description,
                              example: def.example,
                              mana: def.mana,
                              ignition: def.ignition,
                              cooldown: def.cooldown,
                              cardWidth: "86px",
                          });
                          el.style.cssText = "width:100%;height:100%";
                          return el;
                      })()
                    : null;
                flyCard(fr, tr, face, prevSelf.sleeveColor || "blue", 450);
            }
        }

        // 4. Card banished: banishedCards length increased → fly from deck or ignition area to banish zone.
        if (nextSelf.banishedCards.length > prevSelf.banishedCards.length) {
            const toEl = pmEl.banishTopSelf;
            const fromEl = pmEl.ignitionCardSelf;
            const fr = zoneRect(fromEl) || zoneRect(pmEl.deckSelf);
            const tr = zoneRect(toEl);
            if (fr && tr) flyCard(fr, tr, null, nextSelf.sleeveColor || "blue", 500);
        }
        if (nextOpp && prevOpp && nextOpp.banishedCards.length > prevOpp.banishedCards.length) {
            const toEl = pmEl.banishTopOpp;
            const fromEl = pmEl.ignitionCardOpp;
            const fr = zoneRect(fromEl) || zoneRect(pmEl.deckOpp);
            const tr = zoneRect(toEl);
            if (fr && tr) flyCard(fr, tr, null, nextOpp?.sleeveColor || "blue", 500);
        }

        // 5. Card returned to deck: deckCount increased while hand/cooldown decreased → fly from source to deck.
        if (nextSelf.deckCount > prevSelf.deckCount && nextSelf.cooldownCount < prevSelf.cooldownCount) {
            const fromEl = pmEl.cooldownCardsSelf;
            const toEl = pmEl.deckSelf;
            const fr = zoneRect(fromEl);
            const tr = zoneRect(toEl);
            if (fr && tr) flyCard(fr, tr, null, nextSelf.sleeveColor || "blue", 500);
        }

        // 6. Cooldown counters decreased: animate each stacked card badge.
        animateCooldownCounters(prevSelf.cooldownPreview, nextSelf.cooldownPreview, pmEl.cooldownCardsSelf);
        if (prevOpp && nextOpp) {
            animateCooldownCounters(prevOpp.cooldownPreview, nextOpp.cooldownPreview, pmEl.cooldownCardsOpp);
        }
    }

    function animateCardDraw(fromZoneEl, toHandRowEl, isFaceDown, sleeve) {
        const fr = zoneRect(fromZoneEl);
        if (!fr || !toHandRowEl) return;
        const wraps = toHandRowEl.querySelectorAll(".pm-hand-card-wrap");
        const lastSlot = wraps[wraps.length - 1] || toHandRowEl;
        const tr = zoneRect(lastSlot);
        if (!tr) return;
        flyCard(fr, tr, null, sleeve || "blue", 380);
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
     * Animates cooldown turn badges after sort order (matches DOM data-cooldown-index).
     * @param {unknown[]} prevList
     * @param {unknown[]} nextList
     * @param {HTMLElement | null} container
     */
    function animateCooldownCounters(prevList, nextList, container) {
        if (!container || !nextList) return;
        const nextSorted = sortCooldownEntriesForDisplay(nextList);
        const prevSorted = sortCooldownEntriesForDisplay(prevList || []);
        for (let i = 0; i < nextSorted.length; i++) {
            const nextEntry = nextSorted[i];
            const prevEntry = prevSorted.find((p) => p.cardId === nextEntry.cardId);
            if (!prevEntry) continue;
            if (nextEntry.turnsRemaining >= prevEntry.turnsRemaining) continue;
            const wrap = container.querySelector(`[data-cooldown-index="${i}"]`);
            const turnsEl = wrap?.querySelector(".pm-cooldown-turns");
            if (turnsEl) {
                animateCounter(turnsEl, prevEntry.turnsRemaining, nextEntry.turnsRemaining);
            }
        }
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
        const view = isPlaymatUiTestEnabled() ? applyPlaymatUiTestOverlay(snapshot) : snapshot;
        const localPID = String(playerEl.value || "").trim();
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
        updateDrawButton(snapshot, self);
        renderMulliganBar(snapshot);
    }

    /**
     * Shows mulligan instructions, public return counts, and confirm for the local player when the opening phase is active.
     * @param {object} snapshot
     */
    function renderMulliganBar(snapshot) {
        const bar = pmEl.mulliganBar;
        const hint = pmEl.mulliganHint;
        const countsEl = pmEl.mulliganCounts;
        const btn = pmEl.mulliganConfirmBtn;
        if (!bar || !hint || !countsEl || !btn) return;

        const active = !!snapshot.mulliganPhaseActive;
        bar.classList.toggle("hidden", !active);
        if (!active) {
            mulliganPick.clear();
            return;
        }

        hint.textContent = t("mulliganHint");
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
     * Renders cemetery thumbnails. DOM ids are tied to player A/B stacks; when the local
     * player is B, `.board-wrap-perspective-b` reverses the column so the physical stacks
     * swap screen position — we map self/opp pieces and own/opp styling to screen top/bottom.
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
            zs?.setAttribute("aria-label", "Opponent graveyard");
            zo?.setAttribute("aria-label", "Your graveyard");
        } else {
            renderGraveyardGrid(pmEl.graveyardGridSelf, self.graveyardPieces || [], "left");
            renderGraveyardGrid(pmEl.graveyardGridOpp, opp.graveyardPieces || [], "right");
            zs?.classList.remove("pm-graveyard--opp");
            zs?.classList.add("pm-graveyard--own");
            zo?.classList.remove("pm-graveyard--own");
            zo?.classList.add("pm-graveyard--opp");
            zs?.setAttribute("aria-label", "Your graveyard");
            zo?.setAttribute("aria-label", "Opponent graveyard");
        }
    }

    /**
     * Renders graveyard thumbnails in groups by piece type (Q>R>B>N>P), stacked horizontally
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
                const card = createPowerCard({
                    type: def.type,
                    name: def.name,
                    description: def.description,
                    example: def.example,
                    mana: def.manaCost ?? entry.manaCost ?? def.mana,
                    ignition: def.ignition,
                    cooldown: def.cooldown,
                    cardWidth: "220px",
                });
                card.dataset.cardId = entry.cardId;
                wrap.appendChild(card);
            } else {
                const fb = document.createElement("div");
                fb.className = "pm-sleeve-card";
                fb.style.backgroundImage = `url('${sleeveUrl(sleeve)}')`;
                wrap.appendChild(fb);
            }
            stack.appendChild(wrap);
        }

        wireOverlapStackPreviewHover(stack, list, {
            wrapSelector: ".pm-banish-card-wrap",
            peekClassName: "pm-banish-card-wrap--peek",
            hiddenSelector: null,
        });
    }

    function renderIgnitionZone(snapshot) {
        const localPID = playerEl.value;
        // Global ignition slot — show on the owner's side.
        const occupied = snapshot.ignitionOn;
        const owner = snapshot.ignitionOwner;
        const turns = snapshot.ignitionTurnsRemaining ?? 0;
        const cardId = snapshot.ignitionCard;

        const isSelf = owner === localPID;
        const cardEl = isSelf ? pmEl.ignitionCardSelf : pmEl.ignitionCardOpp;
        const counterEl = isSelf ? pmEl.ignitionCounterSelf : pmEl.ignitionCounterOpp;
        const emptyCardEl = isSelf ? pmEl.ignitionCardOpp : pmEl.ignitionCardSelf;
        const emptyCounterEl = isSelf ? pmEl.ignitionCounterOpp : pmEl.ignitionCounterSelf;

        // Clear the other side's ignition.
        if (emptyCardEl) emptyCardEl.innerHTML = "";
        if (emptyCounterEl) emptyCounterEl.classList.add("hidden");

        if (!occupied || !cardEl) return;

        cardEl.innerHTML = "";
        if (counterEl) {
            counterEl.textContent = String(turns);
            counterEl.classList.toggle("hidden", false);
        }

        const def = getCardDef(cardId);
        if (def) {
            const card = createPowerCard({
                type: def.type,
                name: def.name,
                description: def.description,
                example: def.example,
                mana: def.mana,
                ignition: def.ignition,
                cooldown: def.cooldown,
                cardWidth: "220px",
            });
            cardEl.appendChild(card);
            // Hover on container: card has pointer-events:none due to CSS scale transform.
            attachCardHover(cardEl, { ...def, manaCost: def.mana });
        }
    }

    function renderCooldownZone(self, opp) {
        renderCooldownList(pmEl.cooldownCardsSelf, self.cooldownPreview || [], self.cooldownHiddenCount);
        renderCooldownList(pmEl.cooldownCardsOpp, opp.cooldownPreview || [], opp.cooldownHiddenCount);
    }

    /**
     * Renders cooldown as a Balatro-style row: left-aligned, soonest-to-resolve left;
     * overlap tightens so the row fits the container width.
     * @param {HTMLElement | null} container
     * @param {Array<{ cardId: string, turnsRemaining: number }>} allEntries
     * @param {number} hiddenCount
     */
    function renderCooldownList(container, allEntries, hiddenCount) {
        if (!container) return;
        container.innerHTML = "";
        const entries = sortCooldownEntriesForDisplay(allEntries || []);
        const extraHidden = Math.max(0, Number(hiddenCount) || 0);
        const stackTotal = entries.length + (extraHidden > 0 ? 1 : 0);
        container.style.setProperty("--cd-stack-n", String(Math.max(1, stackTotal)));

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

            if (def) {
                const card = createPowerCard({
                    type: def.type,
                    name: def.name,
                    description: def.description,
                    example: def.example,
                    mana: def.mana,
                    ignition: def.ignition,
                    cooldown: def.cooldown,
                    cardWidth: "220px",
                });
                wrap.appendChild(card);
            } else {
                const fb = document.createElement("div");
                fb.className = "pm-sleeve-card";
                wrap.appendChild(fb);
            }

            const turnsEl = document.createElement("span");
            turnsEl.className = "pm-cooldown-turns";
            turnsEl.textContent = `${entry.turnsRemaining}t`;
            wrap.appendChild(turnsEl);
            stack.appendChild(wrap);
        }

        if (extraHidden > 0) {
            const i = entries.length;
            const wrap = document.createElement("div");
            wrap.className = "pm-cooldown-card-wrap pm-cooldown-card-wrap--hidden";
            wrap.dataset.cooldownIndex = String(i);
            wrap.style.setProperty("--i", String(i));
            const label = document.createElement("span");
            label.className = "pm-cooldown-hidden-label";
            label.textContent = `+${extraHidden}`;
            wrap.appendChild(label);
            stack.appendChild(wrap);
        }

        wireCooldownStackPointerHover(stack, entries);
    }

    function renderHandZone(self, opp) {
        renderOwnHand(pmEl.handSelf, self);
        renderOppHand(pmEl.handOpp, opp);
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
        const isMyTurn = snap && snap.turnPlayer === playerEl.value;
        const ignitionOccupied = snap && snap.ignitionOn;
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
                        syncMatchPreviewExample(cid, showing);
                    },
                });
                wrap.appendChild(card);
            }
            wrap.style.setProperty("--i", String(i));
            const canActivate =
                !mulliganChoose && isMyTurn && (!ignitionOccupied || entry.cardId === "save-it-for-later");
            wrap.classList.toggle("pm-hand-card-wrap--inactive", !canActivate);
            wrap.classList.toggle("pm-hand-card-wrap--mulligan-return", mulliganChoose && mulliganPick.has(i));
            if (mulliganChoose) {
                wrap.addEventListener("click", (ev) => {
                    if (ev.target instanceof Element && ev.target.closest(".power-card__toggle")) return;
                    ev.preventDefault();
                    ev.stopPropagation();
                    if (mulliganPick.has(i)) mulliganPick.delete(i);
                    else mulliganPick.add(i);
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
        btn.disabled = mulliganOn || !(isMyTurn && hasMana && hasSpace && gameOn);
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
    let draggingHandEntry = null;

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
            showCardPreview({ ...def, manaCost: def.mana }, wraps[idx]);
        }

        function onLeave() {
            lastIdx = -1;
            hideCardPreview();
            clearPeek();
        }

        stack.addEventListener("mousemove", onMove);
        stack.addEventListener("mouseleave", onLeave);
    }

    /**
     * @param {HTMLElement} stack
     * @param {Array<{ cardId: string, turnsRemaining: number }>} entries
     */
    function wireCooldownStackPointerHover(stack, entries) {
        wireOverlapStackPreviewHover(stack, entries, {
            wrapSelector: ".pm-cooldown-card-wrap:not(.pm-cooldown-card-wrap--hidden)",
            peekClassName: "pm-cooldown-card-wrap--peek",
            hiddenSelector: ".pm-cooldown-card-wrap--hidden",
        });
    }

    /**
     * Pointer-based drag from hand stack to ignition (overlapping hand has pointer-events none on wraps).
     * Hides the large hover preview when a drag starts; lifts the real card (no ghost clone);
     * restores the card to its slot if not dropped on ignition.
     * @param {HTMLElement} stack
     */
    function wireHandStackPointerDrag(stack) {
        const dragThresholdPx = 10;
        /** @type {{ idx: number, entry: object, startX: number, startY: number, pointerId: number, armed: boolean, cardEl?: HTMLElement, wrapEl?: HTMLElement, grabOffsetX?: number, grabOffsetY?: number } | null} */
        let pending = null;

        /**
         * Clears fixed-position drag styling on a hand card and restores the slot placeholder.
         * @param {{ cardEl?: HTMLElement, wrapEl?: HTMLElement } | null} p
         */
        function resetHandDragVisual(p) {
            if (!p?.cardEl || !p.wrapEl) return;
            const card = p.cardEl;
            const wrap = p.wrapEl;
            card.style.position = "";
            card.style.left = "";
            card.style.top = "";
            card.style.width = "";
            card.style.zIndex = "";
            card.style.margin = "";
            card.style.pointerEvents = "";
            card.style.transform = "";
            card.style.transformOrigin = "";
            wrap.classList.remove("pm-hand-card-wrap--dragging");
            wrap.style.minHeight = "";
            stack.style.cursor = "";
        }

        function clearDragChrome() {
            stack.style.cursor = "";
            if (pmEl.ignitionSelf) {
                pmEl.ignitionSelf.classList.remove("pm-drop-active", "pm-drop-hover");
            }
            draggingHandIndex = null;
            draggingHandEntry = null;
        }

        stack.addEventListener("pointerdown", (ev) => {
            if (ev.button !== 0) return;
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
            const isMyTurn = snap && snap.turnPlayer === localPID;
            const ignitionOccupied = snap && snap.ignitionOn;
            const canActivate = isMyTurn && (!ignitionOccupied || entry.cardId === "save-it-for-later");
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
                draggingHandEntry = pending.entry;
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
                    cardEl.style.position = "fixed";
                    cardEl.style.left = `${rect.left}px`;
                    cardEl.style.top = `${rect.top}px`;
                    cardEl.style.width = "220px";
                    cardEl.style.zIndex = "3000";
                    cardEl.style.margin = "0";
                    cardEl.style.pointerEvents = "none";
                    cardEl.style.transform = `scale(${scale})`;
                    cardEl.style.transformOrigin = "top left";
                    stack.style.cursor = "grabbing";
                }
            } else if (pending.cardEl) {
                pending.cardEl.style.left = `${ev.clientX - (pending.grabOffsetX ?? 0)}px`;
                pending.cardEl.style.top = `${ev.clientY - (pending.grabOffsetY ?? 0)}px`;
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
            if (droppedOnIgnition) {
                send("activate_card", { handIndex: pending.idx });
            }
            // Always restore the hand card DOM (clear position:fixed). If we skipped this on
            // ignition drop, a rejected/late snapshot left the card floating forever.
            resetHandDragVisual(pending);
            clearDragChrome();
            pending = null;
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
            if (draggingHandIndex === null) return;
            ev.preventDefault();
            ev.dataTransfer.dropEffect = "move";
        });

        slot.addEventListener("dragleave", (ev) => {
            // Ignore dragleave events that fire when entering a child element.
            if (slot.contains(ev.relatedTarget)) return;
            slot.classList.remove("pm-drop-hover");
        });

        slot.addEventListener("dragenter", (ev) => {
            if (draggingHandIndex === null) return;
            ev.preventDefault();
            slot.classList.add("pm-drop-hover");
        });

        slot.addEventListener("drop", (ev) => {
            ev.preventDefault();
            slot.classList.remove("pm-drop-hover");
            slot.classList.remove("pm-drop-active");
            const idx = draggingHandIndex;
            draggingHandIndex = null;
            draggingHandEntry = null;
            if (idx === null) return;
            send("activate_card", { handIndex: idx });
        });
    }

    setupIgnitionDropTarget();

    // Prevent cards from being dropped anywhere outside the game shell.
    document.addEventListener("dragover", (ev) => {
        if (gameShellEl && !gameShellEl.contains(ev.target) && draggingHandIndex !== null) {
            ev.dataTransfer.dropEffect = "none";
        }
    });

    // Pile view modal (shared for cooldown/banish inspection)
    // showTurns=true renders a "Xt" badge on each card (used for cooldown pile).
    function openPileView(title, cards, sleeve, showTurns) {
        if (!pmEl.pileViewModal || !pmEl.pileViewGrid) return;
        pmEl.pileViewTitle.textContent = title;
        pmEl.pileViewGrid.innerHTML = "";
        if (cards.length === 0) {
            const empty = document.createElement("p");
            empty.style.cssText = "color:#888;text-align:center;width:100%;padding:24px 0";
            empty.textContent = "Empty pile.";
            pmEl.pileViewGrid.appendChild(empty);
        }
        const catalog = getLocalizedCardCatalog(locale);
        const byId = new Map(catalog.map((c) => [c.id, c]));
        for (const entry of cards) {
            const def = byId.get(entry.cardId);
            if (!def) continue;
            const wrap = document.createElement("div");
            wrap.className = "deck-view-card-wrap";
            const card = createPowerCard({
                type: def.type,
                name: def.name,
                description: def.description,
                example: def.example,
                mana: def.mana,
                ignition: def.ignition,
                cooldown: def.cooldown,
                cardWidth: "180px",
            });
            wrap.appendChild(card);
            if (showTurns && entry.turnsRemaining !== undefined) {
                const badge = document.createElement("span");
                badge.className = "count-badge";
                badge.style.color = "#7ab0e0";
                badge.textContent = `${entry.turnsRemaining}t`;
                wrap.appendChild(badge);
            }
            pmEl.pileViewGrid.appendChild(wrap);
        }
        pmEl.pileViewModal.classList.remove("hidden");
        pmEl.pileViewModal.setAttribute("aria-hidden", "false");
    }

    function closePileView() {
        if (!pmEl.pileViewModal) return;
        pmEl.pileViewModal.classList.add("hidden");
        pmEl.pileViewModal.setAttribute("aria-hidden", "true");
        pmEl.pileViewGrid.innerHTML = "";
    }

    if (pmEl.pileViewCloseBtn) {
        pmEl.pileViewCloseBtn.addEventListener("click", closePileView);
    }
    if (pmEl.pileViewModal) {
        pmEl.pileViewModal.addEventListener("click", (ev) => {
            if (ev.target === pmEl.pileViewModal) closePileView();
        });
    }

    // Zone click → open pile modal
    if (pmEl.banishSelf) {
        pmEl.banishSelf.addEventListener("click", () => {
            const snap = lastSnapshot;
            const self = snap?.players?.find((p) => p.playerId === playerEl.value);
            openPileView("Banished — Your pile", self?.banishedCards || []);
        });
    }
    if (pmEl.banishOpp) {
        pmEl.banishOpp.addEventListener("click", () => {
            const snap = lastSnapshot;
            const opp = snap?.players?.find((p) => p.playerId !== playerEl.value);
            openPileView("Banished — Opponent", opp?.banishedCards || []);
        });
    }
    if (pmEl.cooldownSelf) {
        pmEl.cooldownSelf.addEventListener("click", () => {
            const snap = lastSnapshot;
            const self = snap?.players?.find((p) => p.playerId === playerEl.value);
            openPileView("Cooldown — Your pile", self?.cooldownPreview || [], null, true);
        });
    }
    if (pmEl.cooldownOpp) {
        pmEl.cooldownOpp.addEventListener("click", () => {
            const snap = lastSnapshot;
            const opp = snap?.players?.find((p) => p.playerId !== playerEl.value);
            openPileView("Cooldown — Opponent", opp?.cooldownPreview || [], null, true);
        });
    }

    // DRAW button
    if (pmEl.drawBtn) {
        pmEl.drawBtn.addEventListener("click", () => {
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
                if (strikesAEl) strikesAEl.textContent = String(p.strikes ?? 0);
                setBar(manaFillA, manaLabelA, p.mana, p.maxMana);
                setBar(energizedFillA, energizedLabelA, p.energizedMana, p.maxEnergized);
            } else if (p.playerId === "B") {
                if (strikesBEl) strikesBEl.textContent = String(p.strikes ?? 0);
                setBar(manaFillB, manaLabelB, p.mana, p.maxMana);
                setBar(energizedFillB, energizedLabelB, p.energizedMana, p.maxEnergized);
            }
        }
    }

    function clocksActive(snapshot) {
        return snapshot && snapshot.gameStarted === true && !snapshot.matchEnded;
    }

    function renderTurnClocks() {
        if (!clockAEl || !clockBEl) return;
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
        if (!reactionToggleEl || reactionToggleEl.checked) return;
        const rw = snapshot?.reactionWindow;
        if (!rw?.open) return;
        const localPlayer = playerEl.value;
        if (rw.stackSize === 0 && rw.actor && rw.actor !== localPlayer) {
            send("resolve_reactions", {});
        }
    }

    function sendMove(from, to) {
        send("submit_move", {
            fromRow: from.row,
            fromCol: from.col,
            toRow: to.row,
            toCol: to.col,
        });
    }

    function logEvent(obj) {
        if (!eventsEl) return;
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
        if (!boardFrameEl) return;
        syncBoardPerspectiveClass();
        boardFrameEl.innerHTML = "";
        boardFrameEl.classList.toggle("show-inner-coords", coordsInSquaresEl && coordsInSquaresEl.checked);
        const moveSet = new Set(highlightedMoves.map((m) => posKey(m.row, m.col)));
        const selectedKey = selectedFrom ? posKey(selectedFrom.row, selectedFrom.col) : null;
        const ep = lastSnapshot?.enPassant;

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
                        highlightedMoves = computeMoves(
                            lastSnapshot.board,
                            selectedFrom,
                            ep,
                            lastSnapshot?.castlingRights,
                        );
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
                    highlightedMoves = computeMoves(
                        lastSnapshot?.board,
                        draggingFrom,
                        lastSnapshot?.enPassant,
                        lastSnapshot?.castlingRights,
                    );
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
            strike_limit: t("reasonStrikeLimit"),
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
        pmPrevSnapshot = null;
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
                    void connectToRoom(
                        rm.roomId,
                        pieceType,
                        rm.roomName || "Let's Play!",
                        !!rm.roomPrivate,
                        joinPassword,
                    );
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
    if (reactionToggleEl) reactionToggleEl.addEventListener("change", updateReactionToggleLabel);
    if (coordsInSquaresEl) {
        coordsInSquaresEl.addEventListener("change", () => {
            updateCoordsToggleLabel();
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

    globalThis.setInterval(renderTurnClocks, 250);
    renderTurnClocks();
    updatePrivatePasswordVisibility();
    updatePasswordToggleVisual();
    renderBoard([]);
    renderStatus({});
    void bootstrapAuthSession();
    startRoomListPolling();
    document.addEventListener("scroll", () => hideCardPreview(), true);
    window.addEventListener("resize", () => hideCardPreview());
})();
