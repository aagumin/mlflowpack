import pickle
from sklearn.ensemble import RandomForestClassifier
from sklearn.datasets import load_iris

# Train a simple model
X, y = load_iris(return_X_y=True)
model = RandomForestClassifier(n_estimators=10, random_state=42)
model.fit(X, y)

# Save the model
with open('model.pkl', 'wb') as f:
    pickle.dump(model, f)

print("Model saved to model.pkl")
