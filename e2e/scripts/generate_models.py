#!/usr/bin/env python3
"""Generate deterministic MLflow models for local e2e tests."""

from __future__ import annotations

import json
import os
import shutil
from pathlib import Path

import mlflow
import mlflow.pyfunc
import mlflow.sklearn
import numpy as np
import pandas as pd
from sklearn.linear_model import LogisticRegression

ROOT = Path(__file__).resolve().parents[1]
MODELS_DIR = ROOT / "models"


def _is_tracking_enabled() -> bool:
    """Check if MLflow tracking is configured."""
    return os.getenv("MLFLOW_TRACKING_URI") is not None


def _log_to_registry(model_name: str, model, flavor: str, **kwargs) -> None:
    """Log model to MLflow Model Registry if tracking is enabled."""
    if not _is_tracking_enabled():
        return

    mlflow.set_experiment(f"e2e-{model_name}")

    with mlflow.start_run(run_name=f"generate-{model_name}"):
        if flavor == "pyfunc":
            mlflow.pyfunc.log_model(
                artifact_path=model_name,
                python_model=model,
                registered_model_name=model_name,
                **kwargs,
            )
        elif flavor == "sklearn":
            mlflow.sklearn.log_model(
                sk_model=model,
                artifact_path=model_name,
                registered_model_name=model_name,
                **kwargs,
            )

        print(f"Logged model '{model_name}' to MLflow registry")


class SumModel(mlflow.pyfunc.PythonModel):
    def predict(self, context, model_input, params=None):  # noqa: D401
        # Keep prediction behavior deterministic and easy to assert in e2e checks.
        return pd.DataFrame(
            {"sum": model_input["a"].astype(float) + model_input["b"].astype(float)}
        )



def _reset_dir(path: Path) -> None:
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)



def _write_json(path: Path, payload: dict) -> None:
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")



def generate_pyfunc_model() -> None:
    target = MODELS_DIR / "pyfunc"
    _reset_dir(target)

    input_example = pd.DataFrame({"a": [1.5, 2.0], "b": [2.5, 3.0]})
    model = SumModel()

    # Save locally
    mlflow.pyfunc.save_model(
        path=str(target),
        python_model=model,
        input_example=input_example,
    )

    # Log to registry if tracking is enabled
    _log_to_registry(
        model_name="e2e-pyfunc",
        model=model,
        flavor="pyfunc",
        input_example=input_example,
    )

    _write_json(
        target / "test-request.json",
        {
            "inputs": [
                {"name": "a", "shape": [2], "datatype": "FP64", "data": [1.5, 2.0]},
                {"name": "b", "shape": [2], "datatype": "FP64", "data": [2.5, 3.0]},
            ]
        },
    )

    _write_json(
        target / "expected-response.json",
        {
            "output_data": [4.0, 5.0],
            "tolerance": 1e-9,
        },
    )



def generate_sklearn_model() -> None:
    target = MODELS_DIR / "sklearn"
    _reset_dir(target)

    x_train = np.array(
        [
            [0.0, 0.0],
            [0.2, 0.1],
            [0.9, 0.8],
            [1.0, 1.1],
        ]
    )
    y_train = np.array([0, 0, 1, 1])

    model = LogisticRegression(random_state=42)
    model.fit(x_train, y_train)

    input_example = pd.DataFrame({"f1": [0.0, 1.0], "f2": [0.0, 1.0]})

    # Save locally
    mlflow.sklearn.save_model(
        sk_model=model,
        path=str(target),
        input_example=input_example,
    )

    # Log to registry if tracking is enabled
    _log_to_registry(
        model_name="e2e-sklearn",
        model=model,
        flavor="sklearn",
        input_example=input_example,
    )

    _write_json(
        target / "test-request.json",
        {
            "inputs": [
                {
                    "name": "predict",
                    "shape": [2, 2],
                    "datatype": "FP64",
                    "data": [[0.0, 0.0], [1.0, 1.0]],
                }
            ]
        },
    )

    expected = model.predict(np.array([[0.0, 0.0], [1.0, 1.0]])).tolist()
    _write_json(
        target / "expected-response.json",
        {
            "output_data": expected,
            "tolerance": 0,
        },
    )



def main() -> None:
    MODELS_DIR.mkdir(parents=True, exist_ok=True)
    generate_pyfunc_model()
    generate_sklearn_model()


if __name__ == "__main__":
    main()
