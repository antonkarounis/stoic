
const rel = document.getElementById('relationship');
const ageField = document.getElementById('age-field');
const ageInput = document.getElementById('age');
rel.addEventListener('change', () => {
    const isChild = rel.value === 'child';
    ageField.hidden = !isChild;
    ageInput.required = isChild;
});