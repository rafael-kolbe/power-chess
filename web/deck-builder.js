(function () {
  const AUTH_TOKEN_KEY = "powerChessAuthToken";
  const COPY_LIMIT = 3;
  const DECK_SIZE = 20;

  const STR = {
    "en-US": {
      title: "Deck builder",
      back: "← Lobby",
      type: "Card type",
      mana: "Mana cost",
      ign: "Ignition",
      cd: "Cooldown",
      search: "Search name",
      all: "All",
      catalog: "Catalog",
      selDeck: "Selected deck",
      skill: "Player skill",
      sleeve: "Sleeve",
      yourDeck: "Your deck",
      save: "Save",
      newDeck: "Save as new deck",
      clearDeck: "Clear deck",
      deleteDeck: "Delete deck",
      deleteDeckConfirm: "Delete this deck permanently? This cannot be undone.",
      deleteFailed: "Could not delete deck.",
      reloadDeckAria: "Reload deck from server",
      reloadDeckTitle: "Discard unsaved changes and reload this deck from the server",
      newDeckTitle: "New deck",
      deckName: "Deck name",
      cancel: "Cancel",
      confirm: "Confirm",
      count: "{n} / 20",
      needLogin: "Log in from the lobby to build decks.",
      saveFailed: "Could not save.",
      deckNameRequired: "Enter a deck name.",
      skills: [
        ["reinforcements", "Reinforcements"],
        ["march-forward", "March Forward!"],
        ["limitless-potential", "Limitless Potential"],
        ["dimension-shift", "Dimension Shift"]
      ],
      sleeves: [
        ["blue", "Blue"],
        ["green", "Green"],
        ["pink", "Pink"],
        ["red", "Red"]
      ],
      clearFilters: "Clear filters",
      renameDeckTitle: "Rename deck",
      renameDeckSave: "Save name",
      editDeckAria: "Edit deck name",
      editDeckTitle: "Rename the selected deck",
      noticeTitle: "Notice",
      ok: "OK",
      saveNeeds20Cards: "Your deck must have exactly 20 cards to save. You currently have {n}.",
      deleteModalTitle: "Delete deck?",
      confirmDeleteBtn: "Delete"
    },
    "pt-BR": {
      title: "Deck builder",
      back: "← Lobby",
      type: "Tipo da carta",
      mana: "Custo de mana",
      ign: "Ignição",
      cd: "Recarga",
      search: "Buscar nome",
      all: "Todos",
      catalog: "Catálogo",
      selDeck: "Deck selecionado",
      skill: "Habilidade do jogador",
      sleeve: "Sleeve",
      yourDeck: "Seu deck",
      save: "Salvar",
      newDeck: "Salvar como novo deck",
      clearDeck: "Limpar deck",
      deleteDeck: "Deletar deck",
      deleteDeckConfirm: "Excluir este deck permanentemente? Esta ação não pode ser desfeita.",
      deleteFailed: "Não foi possível excluir o deck.",
      reloadDeckAria: "Recarregar deck do servidor",
      reloadDeckTitle: "Descartar alterações não salvas e recarregar este deck do servidor",
      newDeckTitle: "Novo deck",
      deckName: "Nome do deck",
      cancel: "Cancelar",
      confirm: "Confirmar",
      count: "{n} / 20",
      needLogin: "Entre pela lobby para montar decks.",
      saveFailed: "Não foi possível salvar.",
      deckNameRequired: "Informe o nome do deck.",
      skills: [
        ["reinforcements", "Reinforcements"],
        ["march-forward", "March Forward!"],
        ["limitless-potential", "Limitless Potential"],
        ["dimension-shift", "Dimension Shift"]
      ],
      sleeves: [
        ["blue", "Azul"],
        ["green", "Verde"],
        ["pink", "Rosa"],
        ["red", "Vermelho"]
      ],
      clearFilters: "Limpar filtros",
      renameDeckTitle: "Renomear deck",
      renameDeckSave: "Salvar nome",
      editDeckAria: "Editar nome do deck",
      editDeckTitle: "Renomear o deck selecionado",
      noticeTitle: "Aviso",
      ok: "OK",
      saveNeeds20Cards: "O deck precisa ter exatamente 20 cartas para salvar. No momento você tem {n}.",
      deleteModalTitle: "Excluir deck?",
      confirmDeleteBtn: "Excluir"
    }
  };

  function readToken() {
    try {
      return localStorage.getItem(AUTH_TOKEN_KEY) || "";
    } catch (_) {
      return "";
    }
  }

  function authHeaders() {
    const tok = readToken();
    return tok ? { Authorization: `Bearer ${tok}` } : {};
  }

  function getLocale() {
    try {
      return localStorage.getItem("powerChessLocale") || "en-US";
    } catch (_) {
      return "en-US";
    }
  }

  function t(key) {
    const loc = getLocale();
    const dict = STR[loc] || STR["en-US"];
    return dict[key] || STR["en-US"][key] || key;
  }

  function tStr(key, vars) {
    let s = t(key);
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        s = s.replaceAll(`{${k}}`, String(v));
      }
    }
    return s;
  }

  const el = {
    back: document.getElementById("dbBackLink"),
    title: document.getElementById("dbPageTitle"),
    filterType: document.getElementById("dbFilterType"),
    filterMana: document.getElementById("dbFilterMana"),
    filterIgn: document.getElementById("dbFilterIgn"),
    filterCd: document.getElementById("dbFilterCd"),
    filterSearch: document.getElementById("dbFilterSearch"),
    carousel: document.getElementById("dbCarousel"),
    deckSelect: document.getElementById("dbDeckSelect"),
    skillSelect: document.getElementById("dbSkillSelect"),
    sleeveSelect: document.getElementById("dbSleeveSelect"),
    deckGrid: document.getElementById("dbDeckGrid"),
    deckCount: document.getElementById("dbDeckCount"),
    saveBtn: document.getElementById("dbSaveBtn"),
    newDeckBtn: document.getElementById("dbNewDeckBtn"),
    clearDeckBtn: document.getElementById("dbClearDeckBtn"),
    deleteDeckBtn: document.getElementById("dbDeleteDeckBtn"),
    reloadDeckBtn: document.getElementById("dbDeckReloadBtn"),
    renameDeckBtn: document.getElementById("dbDeckRenameBtn"),
    clearFiltersBtn: document.getElementById("dbClearFiltersBtn"),
    hoverPreview: document.getElementById("dbDeckHoverPreview"),
    modal: document.getElementById("dbNewDeckModal"),
    newDeckName: document.getElementById("dbNewDeckNameInput"),
    newDeckCancel: document.getElementById("dbNewDeckCancel"),
    newDeckConfirm: document.getElementById("dbNewDeckConfirm"),
    renameModal: document.getElementById("dbRenameDeckModal"),
    renameDeckInput: document.getElementById("dbRenameDeckNameInput"),
    renameDeckCancel: document.getElementById("dbRenameDeckCancel"),
    renameDeckSave: document.getElementById("dbRenameDeckSave"),
    alertModal: document.getElementById("dbAlertModal"),
    alertTitle: document.getElementById("dbAlertModalTitle"),
    alertMessage: document.getElementById("dbAlertModalMessage"),
    alertOk: document.getElementById("dbAlertModalOk"),
    confirmModal: document.getElementById("dbConfirmModal"),
    confirmTitle: document.getElementById("dbConfirmModalTitle"),
    confirmMessage: document.getElementById("dbConfirmModalMessage"),
    confirmCancel: document.getElementById("dbConfirmModalCancel"),
    confirmDelete: document.getElementById("dbConfirmModalConfirm")
  };

  /** Pointer travel during carousel drag; click-to-add ignores the gesture if above threshold. */
  let carouselDragDistance = 0;
  let lastCarouselGestureTravel = 0;

  /** Remember description vs example toggle per card id in the deck grid (QoL; persists until deck reload). */
  const deckSlotExampleMode = new Map();
  let deckHoverHideTimer = null;
  let deckHoverCardId = null;

  /** @type {string[]} */
  let orderedCardIds = [];
  let selectedDeckId = 0;
  /** @type {{ id: number, name: string }[]} */
  let deckList = [];
  let lobbyDeckId = null;

  function applyStaticLabels() {
    el.title.textContent = t("title");
    el.back.textContent = t("back");
    document.getElementById("dbFilterTypeLabel").textContent = t("type");
    document.getElementById("dbFilterManaLabel").textContent = t("mana");
    document.getElementById("dbFilterIgnLabel").textContent = t("ign");
    document.getElementById("dbFilterCdLabel").textContent = t("cd");
    document.getElementById("dbFilterSearchLabel").textContent = t("search");
    document.getElementById("dbCatalogHeading").textContent = t("catalog");
    document.getElementById("dbSelDeckLabel").textContent = t("selDeck");
    document.getElementById("dbSkillLabel").textContent = t("skill");
    document.getElementById("dbSleeveLabel").textContent = t("sleeve");
    document.getElementById("dbYourDeckHeading").textContent = t("yourDeck");
    el.saveBtn.textContent = t("save");
    el.newDeckBtn.textContent = t("newDeck");
    el.clearDeckBtn.textContent = t("clearDeck");
    el.deleteDeckBtn.textContent = t("deleteDeck");
    el.reloadDeckBtn.setAttribute("aria-label", t("reloadDeckAria"));
    el.reloadDeckBtn.setAttribute("title", t("reloadDeckTitle"));
    el.renameDeckBtn.setAttribute("aria-label", t("editDeckAria"));
    el.renameDeckBtn.setAttribute("title", t("editDeckTitle"));
    document.getElementById("dbRenameDeckTitle").textContent = t("renameDeckTitle");
    document.getElementById("dbRenameDeckNameLabel").textContent = t("deckName");
    el.renameDeckCancel.textContent = t("cancel");
    el.renameDeckSave.textContent = t("renameDeckSave");
    document.getElementById("dbNewDeckTitle").textContent = t("newDeckTitle");
    document.getElementById("dbNewDeckNameLabel").textContent = t("deckName");
    el.newDeckCancel.textContent = t("cancel");
    el.newDeckConfirm.textContent = t("confirm");
    el.clearFiltersBtn.textContent = t("clearFilters");
    el.alertOk.textContent = t("ok");
    el.confirmCancel.textContent = t("cancel");
    el.confirmDelete.textContent = t("confirmDeleteBtn");
  }

  /** @type {(() => void) | null} */
  let alertOnClose = null;
  /** @type {((value: boolean) => void) | null} */
  let confirmResolve = null;

  /**
   * Shows a small notice modal (replaces alert()).
   * @param {string} message
   * @param {{ titleKey?: string, onClose?: () => void }} [opts]
   */
  function openAlertModal(message, opts = {}) {
    const titleKey = opts.titleKey ?? "noticeTitle";
    el.alertTitle.textContent = t(titleKey);
    el.alertMessage.textContent = message;
    alertOnClose = typeof opts.onClose === "function" ? opts.onClose : null;
    el.alertModal.classList.remove("hidden");
    el.alertModal.setAttribute("aria-hidden", "false");
  }

  function closeAlertModal() {
    el.alertModal.classList.add("hidden");
    el.alertModal.setAttribute("aria-hidden", "true");
    const cb = alertOnClose;
    alertOnClose = null;
    if (cb) cb();
  }

  /** @returns {Promise<boolean>} true if user confirms delete */
  function openConfirmDeleteModal() {
    return new Promise((resolve) => {
      confirmResolve = resolve;
      el.confirmTitle.textContent = t("deleteModalTitle");
      el.confirmMessage.textContent = t("deleteDeckConfirm");
      el.confirmModal.classList.remove("hidden");
      el.confirmModal.setAttribute("aria-hidden", "false");
    });
  }

  function closeConfirmModal(result) {
    el.confirmModal.classList.add("hidden");
    el.confirmModal.setAttribute("aria-hidden", "true");
    const r = confirmResolve;
    confirmResolve = null;
    if (r) r(result);
  }

  function fillNumSelect(selectEl, max) {
    selectEl.innerHTML = "";
    const all = document.createElement("option");
    all.value = "";
    all.textContent = t("all");
    selectEl.appendChild(all);
    for (let i = 0; i <= max; i++) {
      const o = document.createElement("option");
      o.value = String(i);
      o.textContent = String(i);
      selectEl.appendChild(o);
    }
  }

  function populateSkillSleeve() {
    el.skillSelect.innerHTML = "";
    el.sleeveSelect.innerHTML = "";
    const sk = STR[getLocale()]?.skills || STR["en-US"].skills;
    const sl = STR[getLocale()]?.sleeves || STR["en-US"].sleeves;
    for (const [id, label] of sk) {
      const o = document.createElement("option");
      o.value = id;
      o.textContent = label;
      el.skillSelect.appendChild(o);
    }
    for (const [id, label] of sl) {
      const o = document.createElement("option");
      o.value = id;
      o.textContent = label;
      el.sleeveSelect.appendChild(o);
    }
  }

  function countMap(ids) {
    const m = new Map();
    for (const id of ids) {
      m.set(id, (m.get(id) || 0) + 1);
    }
    return m;
  }

  function canAdd(cardId) {
    if (orderedCardIds.length >= DECK_SIZE) return false;
    const c = countMap(orderedCardIds);
    return (c.get(cardId) || 0) < COPY_LIMIT;
  }

  function addCard(cardId) {
    if (!canAdd(cardId)) return;
    orderedCardIds.push(cardId);
    renderDeck();
    renderCatalog();
  }

  function removeOne(cardId) {
    for (let i = orderedCardIds.length - 1; i >= 0; i--) {
      if (orderedCardIds[i] === cardId) {
        orderedCardIds.splice(i, 1);
        break;
      }
    }
    deckSlotExampleMode.delete(cardId);
    renderDeck();
    renderCatalog();
  }

  function clearFilters() {
    el.filterType.value = "";
    el.filterMana.value = "";
    el.filterIgn.value = "";
    el.filterCd.value = "";
    el.filterSearch.value = "";
    renderCatalog();
  }

  function hideDeckHoverPreview() {
    if (deckHoverHideTimer) {
      clearTimeout(deckHoverHideTimer);
      deckHoverHideTimer = null;
    }
    el.hoverPreview.classList.add("hidden");
    el.hoverPreview.setAttribute("aria-hidden", "true");
    el.hoverPreview.innerHTML = "";
    deckHoverCardId = null;
  }

  function positionDeckHoverPreview(slotEl) {
    const wrap = el.hoverPreview;
    const pad = 10;
    const w = wrap.offsetWidth;
    const h = wrap.offsetHeight;
    const rect = slotEl.getBoundingClientRect();
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

  /**
   * Syncs the floating preview when the user toggles example/description on the small deck card.
   * @param {string} cardId
   * @param {boolean} showingExample
   */
  function syncDeckHoverExample(cardId, showingExample) {
    if (deckHoverCardId !== cardId) return;
    const article = el.hoverPreview.querySelector(".power-card");
    if (article && typeof globalThis.setPowerCardExampleMode === "function") {
      globalThis.setPowerCardExampleMode(article, showingExample);
    }
  }

  /**
   * @param {HTMLElement} slotEl
   * @param {{ id: string, type: string, name: string, description: string, example: string, mana: number, ignition: number, cooldown: number }} c
   */
  function showDeckHoverPreview(slotEl, c) {
    el.hoverPreview.innerHTML = "";
    const large = createPowerCard({
      type: c.type,
      name: c.name,
      description: c.description,
      example: c.example,
      mana: c.mana,
      ignition: c.ignition,
      cooldown: c.cooldown,
      cardWidth: "260px",
      showExampleInitially: deckSlotExampleMode.get(c.id) === true
    });
    el.hoverPreview.appendChild(large);
    el.hoverPreview.classList.remove("hidden");
    el.hoverPreview.setAttribute("aria-hidden", "false");
    deckHoverCardId = c.id;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => positionDeckHoverPreview(slotEl));
    });
  }

  function filteredCatalog() {
    const locale = getLocale();
    const rows = getLocalizedCardCatalog(locale);
    const type = el.filterType.value;
    const mana = el.filterMana.value;
    const ign = el.filterIgn.value;
    const cd = el.filterCd.value;
    const q = (el.filterSearch.value || "").trim().toLowerCase();
    return rows.filter((r) => {
      if (type && r.type !== type) return false;
      if (mana !== "" && Number(r.mana) !== Number(mana)) return false;
      if (ign !== "" && Number(r.ignition) !== Number(ign)) return false;
      if (cd !== "" && Number(r.cooldown) !== Number(cd)) return false;
      if (q && !String(r.name || "").toLowerCase().includes(q)) return false;
      return true;
    });
  }

  function renderCatalog() {
    el.carousel.innerHTML = "";
    const rows = filteredCatalog().sort(compareCatalogRowsByTypeThenName);
    for (const c of rows) {
      const wrap = document.createElement("div");
      wrap.className = "db-cat-card";
      if (!canAdd(c.id)) wrap.classList.add("db-cat-card--disabled");
      const cardEl = createPowerCard({
        type: c.type,
        name: c.name,
        description: c.description,
        example: c.example,
        mana: c.mana,
        ignition: c.ignition,
        cooldown: c.cooldown,
        cardWidth: "150px"
      });
      wrap.appendChild(cardEl);
      wrap.addEventListener("click", () => {
        if (lastCarouselGestureTravel > 8) {
          lastCarouselGestureTravel = 0;
          return;
        }
        lastCarouselGestureTravel = 0;
        if (canAdd(c.id)) addCard(c.id);
      });
      el.carousel.appendChild(wrap);
    }
  }

  function renderDeck() {
    hideDeckHoverPreview();
    el.deckGrid.innerHTML = "";
    const catalog = getLocalizedCardCatalog(getLocale());
    const byId = new Map(catalog.map((x) => [x.id, x]));
    const counts = countMap(orderedCardIds);
    const sorted = [...counts.keys()].sort((a, b) => compareCardIdsByTypeThenName(a, b, byId));
    el.deckCount.textContent = tStr("count", { n: orderedCardIds.length });

    for (const cid of sorted) {
      const c = byId.get(cid);
      if (!c) continue;
      const n = counts.get(cid);
      const slot = document.createElement("div");
      slot.className = "db-deck-slot";
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
        cardWidth: "100px",
        showExampleInitially: deckSlotExampleMode.get(cid) === true,
        onExampleToggle: (showing) => {
          deckSlotExampleMode.set(cid, showing);
          syncDeckHoverExample(cid, showing);
        }
      });
      slot.appendChild(cardEl);
      slot.appendChild(badge);
      badge.addEventListener("click", (ev) => ev.stopPropagation());
      slot.addEventListener("click", () => removeOne(cid));
      slot.addEventListener("mouseenter", () => {
        if (deckHoverHideTimer) {
          clearTimeout(deckHoverHideTimer);
          deckHoverHideTimer = null;
        }
        showDeckHoverPreview(slot, c);
      });
      slot.addEventListener("mouseleave", () => {
        deckHoverHideTimer = setTimeout(() => hideDeckHoverPreview(), 100);
      });
      el.deckGrid.appendChild(slot);
    }
  }

  function enableDragScroll(container) {
    let down = false;
    let lastPageX = 0;
    container.addEventListener("mousedown", (e) => {
      down = true;
      carouselDragDistance = 0;
      lastCarouselGestureTravel = 0;
      lastPageX = e.pageX;
      container.style.cursor = "grabbing";
    });
    document.addEventListener("mouseup", () => {
      if (down) {
        lastCarouselGestureTravel = carouselDragDistance;
      }
      down = false;
      container.style.cursor = "grab";
    });
    document.addEventListener("mousemove", (e) => {
      if (!down) return;
      e.preventDefault();
      const dx = e.pageX - lastPageX;
      lastPageX = e.pageX;
      carouselDragDistance += Math.abs(dx);
      container.scrollLeft -= dx;
    });
    container.style.cursor = "grab";
  }

  async function loadDeckList() {
    const r = await fetch("/api/decks", { headers: { ...authHeaders(), Accept: "application/json" } });
    if (!r.ok) throw new Error("list");
    const data = await r.json();
    deckList = data.decks || [];
    lobbyDeckId = data.lobbyDeckId ?? null;
    el.deckSelect.innerHTML = "";
    for (const d of deckList) {
      const o = document.createElement("option");
      o.value = String(d.id);
      o.textContent = d.name;
      el.deckSelect.appendChild(o);
    }
    const has = (id) => deckList.some((x) => Number(x.id) === Number(id));
    if (selectedDeckId && has(selectedDeckId)) {
      el.deckSelect.value = String(selectedDeckId);
    } else if (lobbyDeckId != null && has(lobbyDeckId)) {
      el.deckSelect.value = String(lobbyDeckId);
      selectedDeckId = Number(lobbyDeckId);
    } else if (deckList[0]) {
      el.deckSelect.value = String(deckList[0].id);
      selectedDeckId = Number(deckList[0].id);
    } else {
      selectedDeckId = 0;
    }
    syncDeckSelectButtons();
  }

  function syncDeckSelectButtons() {
    const id = Number(el.deckSelect.value, 10);
    const enabled = Boolean(id && deckList.length);
    el.reloadDeckBtn.disabled = !enabled;
    el.renameDeckBtn.disabled = !enabled;
  }

  function openRenameDeckModal() {
    const id = Number(el.deckSelect.value, 10);
    if (!id) return;
    const deck = deckList.find((x) => Number(x.id) === id);
    el.renameDeckInput.value = deck ? deck.name : "";
    el.renameModal.classList.remove("hidden");
    el.renameModal.setAttribute("aria-hidden", "false");
    queueMicrotask(() => {
      el.renameDeckInput.focus();
      el.renameDeckInput.select();
    });
  }

  function closeRenameDeckModal() {
    el.renameModal.classList.add("hidden");
    el.renameModal.setAttribute("aria-hidden", "true");
  }

  async function confirmRenameDeck() {
    const id = Number(el.deckSelect.value, 10);
    if (!id) return;
    const name = (el.renameDeckInput.value || "").trim();
    if (!name) {
      openAlertModal(t("deckNameRequired"));
      return;
    }
    if (orderedCardIds.length !== DECK_SIZE) {
      openAlertModal(tStr("saveNeeds20Cards", { n: orderedCardIds.length }));
      return;
    }
    const r = await fetch(`/api/decks/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json", ...authHeaders() },
      body: JSON.stringify({
        name,
        cardIds: orderedCardIds,
        playerSkillId: el.skillSelect.value,
        sleeveColor: el.sleeveSelect.value
      })
    });
    if (!r.ok) {
      openAlertModal(t("saveFailed"));
      return;
    }
    const entry = deckList.find((x) => Number(x.id) === id);
    if (entry) entry.name = name;
    const opt = el.deckSelect.querySelector(`option[value="${id}"]`);
    if (opt) opt.textContent = name;
    closeRenameDeckModal();
  }

  function clearDeck() {
    orderedCardIds = [];
    deckSlotExampleMode.clear();
    hideDeckHoverPreview();
    renderDeck();
    renderCatalog();
  }

  async function reloadCurrentDeck() {
    const id = Number(el.deckSelect.value, 10);
    if (!id) return;
    try {
      await loadDeckDetails(id);
    } catch (_) {
      openAlertModal(t("saveFailed"));
    }
  }

  async function deleteCurrentDeck() {
    const id = Number(el.deckSelect.value, 10);
    if (!id) return;
    const ok = await openConfirmDeleteModal();
    if (!ok) return;
    const r = await fetch(`/api/decks/${id}`, {
      method: "DELETE",
      headers: { ...authHeaders(), Accept: "application/json" }
    });
    if (!r.ok) {
      openAlertModal(t("deleteFailed"));
      return;
    }
    await loadDeckList();
    if (!deckList.length) {
      orderedCardIds = [];
      deckSlotExampleMode.clear();
      selectedDeckId = 0;
      el.skillSelect.value = "reinforcements";
      el.sleeveSelect.value = "blue";
      hideDeckHoverPreview();
      renderDeck();
      renderCatalog();
      return;
    }
    try {
      await loadDeckDetails(Number(el.deckSelect.value));
    } catch (_) {
      openAlertModal(t("saveFailed"));
    }
  }

  async function loadDeckDetails(id) {
    const r = await fetch(`/api/decks/${id}`, { headers: { ...authHeaders(), Accept: "application/json" } });
    if (!r.ok) throw new Error("deck");
    const d = await r.json();
    orderedCardIds = (d.cardIds || []).slice();
    deckSlotExampleMode.clear();
    el.skillSelect.value = d.playerSkillId || "reinforcements";
    el.sleeveSelect.value = d.sleeveColor || "blue";
    selectedDeckId = Number(id);
    renderDeck();
    renderCatalog();
  }

  async function saveDeck() {
    const id = Number(el.deckSelect.value, 10);
    if (!id) return;
    if (orderedCardIds.length !== DECK_SIZE) {
      openAlertModal(tStr("saveNeeds20Cards", { n: orderedCardIds.length }));
      return;
    }
    const deck = deckList.find((x) => Number(x.id) === id);
    const name = deck ? deck.name : "Deck";
    const r = await fetch(`/api/decks/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json", ...authHeaders() },
      body: JSON.stringify({
        name,
        cardIds: orderedCardIds,
        playerSkillId: el.skillSelect.value,
        sleeveColor: el.sleeveSelect.value
      })
    });
    if (!r.ok) {
      openAlertModal(t("saveFailed"));
      return;
    }
  }

  function openNewModal() {
    el.newDeckName.value = "";
    el.modal.classList.remove("hidden");
    el.modal.setAttribute("aria-hidden", "false");
  }

  function closeNewModal() {
    el.modal.classList.add("hidden");
    el.modal.setAttribute("aria-hidden", "true");
  }

  async function confirmNewDeck() {
    const name = (el.newDeckName.value || "").trim();
    if (!name) {
      openAlertModal(t("deckNameRequired"));
      return;
    }
    if (orderedCardIds.length !== DECK_SIZE) {
      openAlertModal(tStr("saveNeeds20Cards", { n: orderedCardIds.length }));
      return;
    }
    const r = await fetch("/api/decks", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...authHeaders() },
      body: JSON.stringify({
        name,
        cardIds: orderedCardIds.slice(),
        playerSkillId: el.skillSelect.value || "reinforcements",
        sleeveColor: el.sleeveSelect.value || "blue"
      })
    });
    if (!r.ok) {
      openAlertModal(t("saveFailed"));
      return;
    }
    const created = await r.json();
    const newId = Number(created.id);
    await fetch("/api/me/lobby-deck", {
      method: "PUT",
      headers: { "Content-Type": "application/json", ...authHeaders() },
      body: JSON.stringify({ deckId: newId })
    });
    closeNewModal();
    selectedDeckId = newId;
    await loadDeckList();
    await loadDeckDetails(newId);
  }

  async function init() {
    applyStaticLabels();
    el.filterType.options[0].textContent = t("all");

    el.alertOk.addEventListener("click", () => closeAlertModal());
    el.alertModal.addEventListener("click", (ev) => {
      if (ev.target === el.alertModal) closeAlertModal();
    });
    el.confirmCancel.addEventListener("click", () => closeConfirmModal(false));
    el.confirmDelete.addEventListener("click", () => closeConfirmModal(true));
    el.confirmModal.addEventListener("click", (ev) => {
      if (ev.target === el.confirmModal) closeConfirmModal(false);
    });
    document.addEventListener("keydown", (ev) => {
      if (ev.key !== "Escape") return;
      if (!el.renameModal.classList.contains("hidden")) {
        closeRenameDeckModal();
        ev.preventDefault();
        return;
      }
      if (!el.modal.classList.contains("hidden")) {
        closeNewModal();
        ev.preventDefault();
        return;
      }
      if (!el.confirmModal.classList.contains("hidden")) {
        closeConfirmModal(false);
        ev.preventDefault();
        return;
      }
      if (!el.alertModal.classList.contains("hidden")) {
        closeAlertModal();
        ev.preventDefault();
      }
    });

    if (!readToken()) {
      openAlertModal(t("needLogin"), { onClose: () => { location.href = "/"; } });
      return;
    }
    populateSkillSleeve();
    fillNumSelect(el.filterMana, 10);
    fillNumSelect(el.filterIgn, 10);
    fillNumSelect(el.filterCd, 10);

    el.filterSearch.addEventListener("input", () => renderCatalog());
    [el.filterType, el.filterMana, el.filterIgn, el.filterCd].forEach((x) =>
      x.addEventListener("change", () => renderCatalog())
    );
    el.clearFiltersBtn.addEventListener("click", () => clearFilters());

    enableDragScroll(el.carousel);

    document.addEventListener("scroll", () => hideDeckHoverPreview(), true);
    window.addEventListener("resize", () => hideDeckHoverPreview());

    await loadDeckList();
    if (selectedDeckId) {
      await loadDeckDetails(selectedDeckId);
    } else {
      orderedCardIds = [];
      renderDeck();
      renderCatalog();
    }
    el.deckSelect.addEventListener("change", async () => {
      const id = Number(el.deckSelect.value, 10);
      if (id) {
        selectedDeckId = id;
        await loadDeckDetails(id);
      }
      syncDeckSelectButtons();
    });

    el.clearDeckBtn.addEventListener("click", () => clearDeck());
    el.saveBtn.addEventListener("click", () => void saveDeck());
    el.newDeckBtn.addEventListener("click", () => openNewModal());
    el.deleteDeckBtn.addEventListener("click", () => void deleteCurrentDeck());
    el.reloadDeckBtn.addEventListener("click", () => void reloadCurrentDeck());
    el.renameDeckBtn.addEventListener("click", () => openRenameDeckModal());
    el.renameDeckCancel.addEventListener("click", () => closeRenameDeckModal());
    el.renameDeckSave.addEventListener("click", () => void confirmRenameDeck());
    el.renameModal.addEventListener("click", (ev) => {
      if (ev.target === el.renameModal) closeRenameDeckModal();
    });
    el.renameDeckInput.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") {
        ev.preventDefault();
        void confirmRenameDeck();
      }
    });
    el.newDeckCancel.addEventListener("click", () => closeNewModal());
    el.newDeckConfirm.addEventListener("click", () => void confirmNewDeck());
    el.modal.addEventListener("click", (ev) => {
      if (ev.target === el.modal) closeNewModal();
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => void init());
  } else {
    void init();
  }
})();
