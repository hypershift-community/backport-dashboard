document.addEventListener('DOMContentLoaded', function () {
    fetchDocuments();
});

// Global variable to store all documents for filtering
let allDocuments = [];

async function fetchDocuments() {
    try {
        const response = await fetch('/api/documents');

        if (!response.ok) {
            throw new Error(`HTTP error: ${response.status}`);
        }

        allDocuments = await response.json();
        displayDocuments(allDocuments);
        populateAssigneeDropdown(allDocuments);
        setupFilterListeners();
    } catch (error) {
        console.error('Error fetching documents:', error);
        document.getElementById('loading').textContent = 'Error loading documents. Please try again later.';
        document.getElementById('loading').classList.add('error');
    }
}

function populateAssigneeDropdown(documents) {
    const assigneeFilter = document.getElementById('assignee-filter');

    // Extract unique assignees
    const assignees = new Set();
    documents.forEach(doc => {
        if (doc.assignee) {
            assignees.add(doc.assignee);
        }
    });

    // Sort assignees alphabetically
    const sortedAssignees = Array.from(assignees).sort();

    // Add options to dropdown
    sortedAssignees.forEach(assignee => {
        const option = document.createElement('option');
        option.value = assignee;
        option.textContent = assignee;
        assigneeFilter.appendChild(option);
    });
}

function setupFilterListeners() {
    const assigneeFilter = document.getElementById('assignee-filter');
    const showCompletedCheckbox = document.getElementById('show-completed');

    assigneeFilter.addEventListener('change', function () {
        filterCardsByAssignee(this.value);
    });

    showCompletedCheckbox.addEventListener('change', function () {
        toggleCompletedCards();
    });
}

function filterCardsByAssignee(assignee) {
    const cards = document.querySelectorAll('.card');

    cards.forEach(card => {
        // Remove previous filter if any
        card.classList.remove('filtered');

        // If "all" is selected, show all cards
        if (assignee === 'all') {
            return;
        }

        // Find the assignee field by looking for the card-info with "Assignee:" label
        let cardAssignee = null;
        const cardInfos = card.querySelectorAll('.card-info');

        for (const info of cardInfos) {
            const label = info.querySelector('.card-info-label');
            if (label && label.textContent.trim() === 'Assignee:') {
                cardAssignee = info.querySelector('.card-info-value').textContent;
                break;
            }
        }

        // Hide the card if it doesn't match the selected assignee
        if (cardAssignee !== assignee) {
            card.classList.add('filtered');
        }
    });
}

function displayDocuments(documents) {
    const cardsContainer = document.getElementById('cards-container');
    const loadingElement = document.getElementById('loading');

    // Clear any existing cards
    cardsContainer.innerHTML = '';

    if (documents.length === 0) {
        loadingElement.textContent = 'No documents found.';
        return;
    }

    loadingElement.style.display = 'none';

    documents.forEach(doc => {
        cardsContainer.appendChild(createCard(doc));
    });
}

function createCard(doc) {
    const card = document.createElement('div');
    card.className = 'card';

    // Add completed class if the document is marked as complete
    if (doc.completed) {
        card.classList.add('completed');
        card.style.display = document.getElementById('show-completed')?.checked ? 'block' : 'none';
    }

    // Card header with ID, summary, and buttons
    const cardHeader = document.createElement('div');
    cardHeader.className = 'card-header';

    // Add header content wrapper
    const headerContent = document.createElement('div');
    headerContent.className = 'header-content';

    const cardTitle = document.createElement('h2');
    cardTitle.className = 'card-title';
    cardTitle.innerHTML = `<a href="https://issues.redhat.com/browse/${doc._id}" class="link" target="_blank">${doc._id}</a>`;

    headerContent.appendChild(cardTitle);

    // Create buttons container
    const buttonsContainer = document.createElement('div');
    buttonsContainer.className = 'buttons-container';

    // Add Complete/Incomplete toggle button
    const completeButton = document.createElement('button');
    completeButton.className = doc.completed ? 'incomplete-button' : 'complete-button';
    completeButton.textContent = doc.completed ? 'Mark as Incomplete' : 'Mark as Complete';
    completeButton.title = doc.completed ? 'Mark this card as incomplete' : 'Mark this card as complete';
    completeButton.addEventListener('click', function () {
        toggleCompletion(doc._id, card, !doc.completed);
    });

    // Add close button
    const closeButton = document.createElement('button');
    closeButton.className = 'close-button';
    closeButton.innerHTML = '✕';
    closeButton.title = 'Close this card';
    closeButton.addEventListener('click', function () {
        card.style.display = 'none';
    });

    // Add buttons to container
    buttonsContainer.appendChild(completeButton);
    buttonsContainer.appendChild(closeButton);

    cardHeader.appendChild(headerContent);
    cardHeader.appendChild(buttonsContainer);
    card.appendChild(cardHeader);

    // Card body with main info
    const cardBody = document.createElement('div');
    cardBody.className = 'card-body';

    // Summary information
    const cardSubtitle = document.createElement('p');
    cardSubtitle.className = 'card-subtitle';
    cardSubtitle.textContent = doc.summary || 'No summary available';
    cardBody.appendChild(cardSubtitle);

    // Status information
    const statusInfo = createInfoItem('Status', doc.status);
    statusInfo.querySelector('.card-info-value').classList.add('status-badge', `status-${doc.status ? doc.status.toLowerCase() : 'unknown'}`);
    cardBody.appendChild(statusInfo);

    // Target version
    cardBody.appendChild(createInfoItem('Target Version', doc.target_version || 'N/A'));

    // Target backport versions
    cardBody.appendChild(createInfoItem('Backport Versions', doc.target_backport_versions));

    // Assignee information
    cardBody.appendChild(createInfoItem('Assignee', doc.assignee || 'Unassigned'));

    // Check for missing backports and add warnings if necessary
    if (doc.target_backport_versions) {
        const missingBackports = findMissingBackports(doc);
        if (missingBackports.length > 0) {
            cardBody.appendChild(createBackportWarnings(missingBackports));
        }
    }

    card.appendChild(cardBody);

    // Add clones section if available
    if (doc.clone) {
        const clonesSection = createClonesSection(doc.clone);
        card.appendChild(clonesSection);
    }

    return card;
}

async function toggleCompletion(documentId, card, setCompleted) {
    try {
        const response = await fetch('/api/documents/complete', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                id: documentId,
                completed: setCompleted
            })
        });

        if (!response.ok) {
            throw new Error(`HTTP error: ${response.status}`);
        }

        const result = await response.json();

        if (result.success) {
            // Update the document in our local cache
            const docIndex = allDocuments.findIndex(doc => doc._id === documentId);
            if (docIndex !== -1) {
                allDocuments[docIndex].completed = setCompleted;
            }

            if (setCompleted) {
                // Mark as complete
                card.classList.add('completed');

                // Hide the card if show completed is not checked
                const showCompleted = document.getElementById('show-completed').checked;
                if (!showCompleted) {
                    card.style.display = 'none';
                }

                // Update button
                const completeButton = card.querySelector('.complete-button');
                if (completeButton) {
                    completeButton.textContent = 'Mark as Incomplete';
                    completeButton.title = 'Mark this card as incomplete';
                    completeButton.className = 'incomplete-button';
                }
            } else {
                // Mark as incomplete
                card.classList.remove('completed');
                card.style.display = 'block';

                // Update button
                const incompleteButton = card.querySelector('.incomplete-button');
                if (incompleteButton) {
                    incompleteButton.textContent = 'Mark as Complete';
                    incompleteButton.title = 'Mark this card as complete';
                    incompleteButton.className = 'complete-button';
                }
            }
        }
    } catch (error) {
        console.error('Error updating document completion status:', error);
        alert('Failed to update document status. Please try again.');
    }
}

// Add a method to toggle visibility of completed cards
function toggleCompletedCards() {
    const completedCards = document.querySelectorAll('.card.completed');
    const showCompleted = document.getElementById('show-completed').checked;

    completedCards.forEach(card => {
        card.style.display = showCompleted ? 'block' : 'none';
    });
}

function createInfoItem(label, value) {
    const infoDiv = document.createElement('div');
    infoDiv.className = 'card-info';

    const labelSpan = document.createElement('span');
    labelSpan.className = 'card-info-label';
    labelSpan.textContent = label + ':';

    const valueSpan = document.createElement('span');
    valueSpan.className = 'card-info-value';
    valueSpan.textContent = value;

    infoDiv.appendChild(labelSpan);
    infoDiv.appendChild(valueSpan);

    return infoDiv;
}

function createClonesSection(clone, depth = 0) {
    const container = document.createElement('div');
    container.className = depth === 0 ? 'card-clones' : 'clone-item';

    if (depth === 0) {
        const title = document.createElement('h3');
        title.textContent = 'Backports';
        container.appendChild(title);
    }

    const cloneInfo = document.createElement('div');
    cloneInfo.className = 'clone-info';

    // Clone ID if available (not in the first level)
    if (clone._id) {
        const idLink = document.createElement('a');
        idLink.href = `https://issues.redhat.com/browse/${clone._id}`;
        idLink.className = 'link';
        idLink.textContent = clone._id;
        idLink.target = '_blank';

        const idDiv = document.createElement('div');
        idDiv.className = 'clone-id';
        idDiv.appendChild(idLink);
        cloneInfo.appendChild(idDiv);
    }

    // Status and target version
    const detailsDiv = document.createElement('div');
    detailsDiv.className = 'clone-details';

    if (clone.status) {
        const statusBadge = document.createElement('span');
        statusBadge.className = `status-badge status-${clone.status.toLowerCase()}`;
        statusBadge.textContent = clone.status;
        detailsDiv.appendChild(statusBadge);
    }

    if (clone.target_version) {
        const versionSpan = document.createElement('span');
        versionSpan.className = 'clone-version';
        versionSpan.textContent = clone.target_version;
        detailsDiv.appendChild(versionSpan);
    }

    cloneInfo.appendChild(detailsDiv);
    container.appendChild(cloneInfo);

    // Recursively add nested clones
    if (clone.clone) {
        container.appendChild(createClonesSection(clone.clone, depth + 1));
    }

    return container;
}

// Functions to handle backport version verification
function findMissingBackports(doc) {
    // Get all target versions from clones
    const cloneVersions = new Set();
    if (doc.clone) {
        collectCloneVersions(doc.clone, cloneVersions);
    }

    // Check which backport versions don't have corresponding clones
    const missingBackports = [];
    if (doc.target_backport_versions) {
        const backportVersions = doc.target_backport_versions.split(', ');
        backportVersions.forEach(version => {
            if (version && !cloneVersions.has(version)) {
                missingBackports.push(version);
            }
        });
    }

    return missingBackports;
}

function collectCloneVersions(clone, versionSet) {
    if (clone.target_version) {
        versionSet.add(clone.target_version);
    }

    // Recursively check nested clones
    if (clone.clone) {
        collectCloneVersions(clone.clone, versionSet);
    }
}

function createBackportWarnings(missingVersions) {
    const warningContainer = document.createElement('div');
    warningContainer.className = 'backport-warnings';

    const warningHeader = document.createElement('h4');
    warningHeader.className = 'warning-header';
    warningHeader.textContent = 'Missing Backports:';
    warningContainer.appendChild(warningHeader);

    missingVersions.forEach(version => {
        const listItem = document.createElement('p');
        listItem.className = 'warning-item';
        listItem.textContent = `Missing backport for version ${version}`;
        warningContainer.appendChild(listItem);
    });

    return warningContainer;
}