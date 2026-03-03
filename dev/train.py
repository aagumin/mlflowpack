"""
Sample training script for testing the MLflow buildpack.
Trains a simple sklearn model and registers it with MLflow.
"""
import mlflow
from sklearn.datasets import load_iris
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
from sklearn.metrics import accuracy_score
import yaml
import os


def train():
    # Load data
    X, y = load_iris(return_X_y=True)
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

    # Train model
    model = RandomForestClassifier(n_estimators=100, random_state=42)
    model.fit(X_train, y_train)

    # Evaluate
    y_pred = model.predict(X_test)
    accuracy = accuracy_score(y_test, y_pred)
    print(f"Accuracy: {accuracy:.4f}")

    # Configure MLflow
    mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://localhost:5000"))
    mlflow.set_experiment("iris-classification")

    # Log model
    with mlflow.start_run():
        mlflow.log_metric("accuracy", accuracy)

        # Log model with conda environment
        mlflow.sklearn.log_model(
            model,
            "model",
            registered_model_name="iris-classifier",
            conda_env={
                "channels": ["defaults", "conda-forge"],
                "dependencies": [
                    "python=3.10.13",
                    {"pip": ["scikit-learn==1.3.0", "mlserver==1.7.1", "mlserver-sklearn"]}
                ]
            }
        )

        print(f"Model registered: iris-classifier")
        print(f"Run ID: {mlflow.active_run().info.run_id}")


if __name__ == "__main__":
    train()
