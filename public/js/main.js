/**
 * AWEN — Client Interactions
 * Monochromatic, Refined, Accessible
 */

(function () {
  'use strict';

  // ---------------------------------------------------------------------------
  // 1. Theme Toggle (Light / Dark)
  // ---------------------------------------------------------------------------
  function initTheme() {
    const toggleBtn = document.getElementById('theme-toggle');
    if (!toggleBtn) return;

    function getSystemTheme() {
      return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    function getCurrentTheme() {
      return document.documentElement.getAttribute('data-theme') || localStorage.getItem('awen-theme') || getSystemTheme();
    }

    function applyTheme(theme) {
      document.documentElement.setAttribute('data-theme', theme);
      localStorage.setItem('awen-theme', theme);
      updateThemeIcon(theme);
    }

    function updateThemeIcon(theme) {
      const sunIcon = toggleBtn.querySelector('.icon-sun');
      const moonIcon = toggleBtn.querySelector('.icon-moon');
      if (!sunIcon || !moonIcon) return;

      if (theme === 'dark') {
        sunIcon.style.display = 'block';
        moonIcon.style.display = 'none';
        toggleBtn.setAttribute('aria-label', 'Switch to light mode');
      } else {
        sunIcon.style.display = 'none';
        moonIcon.style.display = 'block';
        toggleBtn.setAttribute('aria-label', 'Switch to dark mode');
      }
    }

    // Initialize state
    const current = getCurrentTheme();
    updateThemeIcon(current);

    toggleBtn.addEventListener('click', () => {
      const active = getCurrentTheme();
      const next = active === 'dark' ? 'light' : 'dark';
      applyTheme(next);
    });

    // Listen to OS theme changes if user has not explicitly set a manual preference
    if (window.matchMedia) {
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (!localStorage.getItem('awen-theme')) {
          applyTheme(e.matches ? 'dark' : 'light');
        }
      });
    }
  }

  // ---------------------------------------------------------------------------
  // 2. Search & Tag Filtering (Full Static & Dynamic support)
  // ---------------------------------------------------------------------------
  function initSearchAndFilter() {
    const searchInput = document.getElementById('search-input');
    const searchClear = document.getElementById('search-clear');
    const tagPills = document.querySelectorAll('.tag-pill');
    const articlesContainer = document.getElementById('articles-stream');
    const filterStatus = document.getElementById('filter-status');
    const filterStatusText = document.getElementById('filter-status-text');
    const filterResetBtn = document.getElementById('filter-reset-btn');
    const paginationNav = document.getElementById('pagination-nav');

    if (!articlesContainer) return;

    let searchIndex = null;
    let currentTag = '';
    let currentQuery = '';

    // Cache pre-rendered cards
    const initialCards = Array.from(articlesContainer.querySelectorAll('.article-card'));

    // Fetch search index for full text search
    async function loadSearchIndex() {
      if (searchIndex) return searchIndex;
      try {
        const res = await fetch('/search.json');
        if (res.ok) {
          searchIndex = await res.json();
          return searchIndex;
        }
      } catch (err) {
        console.warn('Search index load failed; falling back to DOM cards:', err);
      }
      return null;
    }

    function updateURLState() {
      const params = new URLSearchParams();
      if (currentQuery) params.set('q', currentQuery);
      if (currentTag) params.set('tag', currentTag);
      const newQuery = params.toString() ? '?' + params.toString() : window.location.pathname;
      window.history.replaceState({}, '', newQuery);
    }

    function renderFilteredCards(matches, isFiltered) {
      if (!isFiltered) {
        // Restore initial pre-rendered cards
        articlesContainer.innerHTML = '';
        initialCards.forEach(c => articlesContainer.appendChild(c));
        if (filterStatus) filterStatus.classList.remove('visible');
        if (paginationNav) paginationNav.style.display = '';
        return;
      }

      // Hide default pagination during search/filter
      if (paginationNav) paginationNav.style.display = 'none';

      if (filterStatus) {
        filterStatus.classList.add('visible');
        let statusMsg = `Found ${matches.length} article${matches.length === 1 ? '' : 's'}`;
        if (currentTag) statusMsg += ` in tag "${currentTag}"`;
        if (currentQuery) statusMsg += ` matching "${currentQuery}"`;
        filterStatusText.textContent = statusMsg;
      }

      if (matches.length === 0) {
        articlesContainer.innerHTML = `
          <div class="empty-comments" style="margin: 2rem 0; padding: 3rem 1rem;">
            No articles found matching your criteria. Try adjusting your search query or selected tag.
          </div>
        `;
        return;
      }

      articlesContainer.innerHTML = '';
      matches.forEach(item => {
        const card = document.createElement('article');
        card.className = 'article-card';
        card.setAttribute('data-slug', item.slug);

        const tagsHtml = (item.tags || []).map(t => `<span class="card-tag">#${escapeHtml(t)}</span>`).join('');

        card.innerHTML = `
          <div class="card-meta">
            <time datetime="${escapeHtml(item.date)}">${escapeHtml(item.date_formatted || item.date)}</time>
            <span class="meta-dot">·</span>
            <span>${escapeHtml(item.reading_time || '4 min')}</span>
          </div>
          <h2 class="card-title">
            <a href="/blog/${encodeURIComponent(item.slug)}/">${escapeHtml(item.title)}</a>
          </h2>
          <p class="card-summary">${escapeHtml(item.summary || '')}</p>
          <div class="card-footer">
            <div class="card-tags">${tagsHtml}</div>
            <div class="card-stats" data-slug-stats="${encodeURIComponent(item.slug)}">
              <span class="stat-item claps-stat">
                <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 14h-2v-2h2v2zm0-4h-2V7h2v5z"/></svg>
                <span class="stat-num">${item.claps || 0}</span> claps
              </span>
            </div>
          </div>
        `;
        articlesContainer.appendChild(card);
      });

      // Update live claps stats for newly rendered cards
      fetchArticleStatsForCards();
    }

    async function applyFilters() {
      const q = currentQuery.trim().toLowerCase();
      const tag = currentTag.trim().toLowerCase();
      const isFiltered = Boolean(q || tag);

      if (searchClear) {
        if (q) searchClear.classList.add('visible');
        else searchClear.classList.remove('visible');
      }

      // Update tag pill active classes
      tagPills.forEach(pill => {
        const pillTag = pill.getAttribute('data-tag') || '';
        if (tag && pillTag.toLowerCase() === tag) {
          pill.classList.add('active');
        } else if (!tag && pillTag === '') {
          pill.classList.add('active');
        } else {
          pill.classList.remove('active');
        }
      });

      updateURLState();

      if (!isFiltered) {
        renderFilteredCards([], false);
        return;
      }

      const index = await loadSearchIndex();

      let results = [];
      if (index && Array.isArray(index)) {
        results = index.filter(item => {
          const matchTag = !tag || (item.tags && item.tags.some(t => t.toLowerCase() === tag));
          if (!matchTag) return false;
          if (!q) return true;

          const titleMatch = item.title && item.title.toLowerCase().includes(q);
          const summaryMatch = item.summary && item.summary.toLowerCase().includes(q);
          const contentMatch = item.content && item.content.toLowerCase().includes(q);
          const tagMatch = item.tags && item.tags.some(t => t.toLowerCase().includes(q));
          return titleMatch || summaryMatch || contentMatch || tagMatch;
        });
      } else {
        // Fallback filter using existing DOM cards
        initialCards.forEach(card => {
          const title = (card.querySelector('.card-title')?.textContent || '').toLowerCase();
          const summary = (card.querySelector('.card-summary')?.textContent || '').toLowerCase();
          const tags = Array.from(card.querySelectorAll('.card-tag')).map(el => el.textContent.replace('#', '').toLowerCase());

          const matchTag = !tag || tags.includes(tag);
          if (!matchTag) return;
          if (!q) {
            results.push({ slug: card.getAttribute('data-slug') });
            return;
          }
          if (title.includes(q) || summary.includes(q) || tags.some(t => t.includes(q))) {
            results.push({ slug: card.getAttribute('data-slug') });
          }
        });
      }

      renderFilteredCards(results, true);
    }

    // Input event handlers
    if (searchInput) {
      searchInput.addEventListener('input', (e) => {
        currentQuery = e.target.value;
        applyFilters();
      });

      // Quick focus shortcut ('/' or Cmd+K)
      window.addEventListener('keydown', (e) => {
        if (e.key === '/' && document.activeElement !== searchInput && document.activeElement.tagName !== 'TEXTAREA') {
          e.preventDefault();
          searchInput.focus();
        } else if (e.key === 'Escape' && document.activeElement === searchInput) {
          searchInput.blur();
        }
      });
    }

    if (searchClear) {
      searchClear.addEventListener('click', () => {
        if (searchInput) {
          searchInput.value = '';
          searchInput.focus();
        }
        currentQuery = '';
        applyFilters();
      });
    }

    tagPills.forEach(pill => {
      pill.addEventListener('click', () => {
        const clickedTag = pill.getAttribute('data-tag') || '';
        if (currentTag.toLowerCase() === clickedTag.toLowerCase()) {
          // Toggle off if already active
          currentTag = '';
        } else {
          currentTag = clickedTag;
        }
        applyFilters();
      });
    });

    if (filterResetBtn) {
      filterResetBtn.addEventListener('click', () => {
        currentQuery = '';
        currentTag = '';
        if (searchInput) searchInput.value = '';
        applyFilters();
      });
    }

    // Read initial filters from URL params
    const initialParams = new URLSearchParams(window.location.search);
    if (initialParams.has('q')) {
      currentQuery = initialParams.get('q');
      if (searchInput) searchInput.value = currentQuery;
    }
    if (initialParams.has('tag')) {
      currentTag = initialParams.get('tag');
    }

    if (currentQuery || currentTag) {
      applyFilters();
    }
  }

  // ---------------------------------------------------------------------------
  // 3. Claps Interaction (Unlimited Claps)
  // ---------------------------------------------------------------------------
  function initClaps() {
    const clapBtn = document.getElementById('clap-button');
    if (!clapBtn) return;

    const slug = clapBtn.getAttribute('data-slug');
    const clapCountEl = clapBtn.querySelector('.clap-count');
    if (!slug || !clapCountEl) return;

    let currentClaps = parseInt(clapCountEl.textContent, 10) || 0;
    let pendingClaps = 0;
    let sendTimeout = null;

    // Fetch initial live count from backend API if available
    fetch(`/api/articles/${encodeURIComponent(slug)}/data`)
      .then(res => res.ok ? res.json() : null)
      .then(data => {
        if (data && typeof data.claps === 'number') {
          currentClaps = data.claps;
          clapCountEl.textContent = currentClaps;
        }
      })
      .catch(() => {
        // Fall back to server-rendered count quietly
      });

    function spawnBubble() {
      const bubble = document.createElement('span');
      bubble.className = 'clap-bubble';
      bubble.textContent = '+1';
      clapBtn.parentElement.appendChild(bubble);

      setTimeout(() => {
        bubble.remove();
      }, 800);
    }

    async function flushClaps() {
      if (pendingClaps <= 0) return;
      const countToSend = pendingClaps;
      pendingClaps = 0;

      try {
        const res = await fetch(`/api/articles/${encodeURIComponent(slug)}/clap`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ count: countToSend })
        });
        if (res.ok) {
          const result = await res.json();
          if (result && typeof result.claps === 'number') {
            currentClaps = result.claps;
            clapCountEl.textContent = currentClaps;
          }
        }
      } catch (err) {
        console.warn('Failed to sync claps with server:', err);
      }
    }

    clapBtn.addEventListener('click', () => {
      // Optimistic instant UI update
      currentClaps += 1;
      pendingClaps += 1;
      clapCountEl.textContent = currentClaps;
      spawnBubble();

      // Tactile button bounce
      clapBtn.style.transform = 'scale(0.94)';
      setTimeout(() => {
        clapBtn.style.transform = '';
      }, 100);

      // Debounced batch sync to avoid network choke on rapid clicks
      clearTimeout(sendTimeout);
      sendTimeout = setTimeout(flushClaps, 400);
    });
  }

  // ---------------------------------------------------------------------------
  // 4. Comments Feature (Clean, Minimalist, Thread-safe)
  // ---------------------------------------------------------------------------
  function initComments() {
    const commentsContainer = document.getElementById('comments-container');
    const commentForm = document.getElementById('comment-form');
    if (!commentsContainer || !commentForm) return;

    const slug = commentForm.getAttribute('data-slug');
    const authorInput = document.getElementById('comment-author');
    const contentInput = document.getElementById('comment-content');
    const submitBtn = document.getElementById('comment-submit');
    const statusEl = document.getElementById('comment-status');
    const countPill = document.getElementById('comments-count-pill');

    if (!slug) return;

    function getInitials(name) {
      if (!name) return 'A';
      const parts = name.trim().split(/\s+/);
      if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
    }

    function createCommentElement(comment) {
      const card = document.createElement('div');
      card.className = 'comment-card';
      card.setAttribute('data-id', comment.id || '');

      const initials = getInitials(comment.author);
      const dateStr = formatDate(comment.created_at);

      card.innerHTML = `
        <div class="comment-meta">
          <div class="comment-author-info">
            <span class="comment-avatar">${escapeHtml(initials)}</span>
            <span class="comment-author-name">${escapeHtml(comment.author)}</span>
          </div>
          <time class="comment-date" datetime="${escapeHtml(comment.created_at)}">${escapeHtml(dateStr)}</time>
        </div>
        <div class="comment-body">${escapeHtml(comment.content)}</div>
      `;
      return card;
    }

    function updateCount(count) {
      if (countPill) {
        countPill.textContent = `${count} comment${count === 1 ? '' : 's'}`;
      }
    }

    // Fetch latest comments from server
    async function loadLatestComments() {
      try {
        const res = await fetch(`/api/articles/${encodeURIComponent(slug)}/data`);
        if (!res.ok) return;
        const data = await res.json();
        if (data && Array.isArray(data.comments)) {
          const list = document.getElementById('comments-list');
          if (!list) return;
          list.innerHTML = '';
          if (data.comments.length === 0) {
            list.innerHTML = `<div class="empty-comments">No reflections yet. Be the first to share your thoughts.</div>`;
          } else {
            data.comments.forEach(c => list.appendChild(createCommentElement(c)));
          }
          updateCount(data.comments.length);
        }
      } catch (e) {
        // Silently preserve pre-rendered HTML comments
      }
    }

    commentForm.addEventListener('submit', async (e) => {
      e.preventDefault();

      const author = (authorInput.value || '').trim();
      const content = (contentInput.value || '').trim();

      if (!author || !content) {
        if (statusEl) {
          statusEl.textContent = 'Please enter both your name and reflection.';
          statusEl.style.color = '#e53e3e';
        }
        return;
      }

      submitBtn.disabled = true;
      if (statusEl) {
        statusEl.textContent = 'Submitting reflection...';
        statusEl.style.color = 'var(--text-muted)';
      }

      try {
        const res = await fetch(`/api/articles/${encodeURIComponent(slug)}/comments`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ author, content })
        });

        if (!res.ok) {
          throw new Error(`Server returned ${res.status}`);
        }

        const data = await res.json();
        if (data && data.comment) {
          // Success
          contentInput.value = '';
          if (statusEl) {
            statusEl.textContent = 'Reflection published.';
            statusEl.style.color = 'var(--text-secondary)';
            setTimeout(() => { statusEl.textContent = ''; }, 3000);
          }

          const list = document.getElementById('comments-list');
          if (list) {
            const emptyNotice = list.querySelector('.empty-comments');
            if (emptyNotice) emptyNotice.remove();

            const newCard = createCommentElement(data.comment);
            list.appendChild(newCard);

            const allCards = list.querySelectorAll('.comment-card');
            updateCount(allCards.length);
          }
        }
      } catch (err) {
        console.error('Failed to post comment:', err);
        if (statusEl) {
          statusEl.textContent = 'Error publishing comment. Please try again.';
          statusEl.style.color = '#e53e3e';
        }
      } finally {
        submitBtn.disabled = false;
      }
    });

    // Initial refresh
    loadLatestComments();
  }

  // ---------------------------------------------------------------------------
  // 5. Utility: Fetch Stats on Card Stream
  // ---------------------------------------------------------------------------
  async function fetchArticleStatsForCards() {
    const statsNodes = document.querySelectorAll('[data-slug-stats]');
    if (statsNodes.length === 0) return;

    statsNodes.forEach(async node => {
      const slug = node.getAttribute('data-slug-stats');
      if (!slug) return;
      try {
        const res = await fetch(`/api/articles/${encodeURIComponent(slug)}/data`);
        if (!res.ok) return;
        const data = await res.json();
        if (data && typeof data.claps === 'number') {
          const clapsNum = node.querySelector('.stat-num');
          if (clapsNum) clapsNum.textContent = data.claps;
        }
      } catch (e) {
        // keep pre-rendered count
      }
    });
  }

  // ---------------------------------------------------------------------------
  // 6. Helpers
  // ---------------------------------------------------------------------------
  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  function formatDate(isoStr) {
    if (!isoStr) return '';
    try {
      const date = new Date(isoStr);
      return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric'
      });
    } catch {
      return isoStr;
    }
  }

  // ---------------------------------------------------------------------------
  // Bootstrap on DOM ready
  // ---------------------------------------------------------------------------
  document.addEventListener('DOMContentLoaded', () => {
    initTheme();
    initSearchAndFilter();
    initClaps();
    initComments();
    fetchArticleStatsForCards();
  });
})();
