#!/usr/bin/env python3
"""Create a test sklearn model for kpack tests."""
import pickle
import yaml
import os
from sklearn.ensemble import RandomForestClassifier
from sklearn.datasets import load_iris

def create_test_model():
    # Create output directory
    model_dir = os.path.dirname(os.path.abspath(__file__))

    # Train a simple model
    X, y = load_iris(return_X_y=True)
    model = RandomForestClassifier(n_estimators=10, random_state=42)
    model.fit(X, y)

    # Save model
    with open(os.path.join(model_dir, 'model.pkl'), 'wb') as f:
        pickle.dump(model, f)

    # Create MLmodel file
    mlmodel = {
        'artifact_path': 'model',
        'flavors': {
            'sklearn': {
                'sklearn_version': '1.3.0',
                'pickled_model': 'model.pkl',
                'code': None,
            },
            'python_function': {
                'loader_module': 'mlflow.sklearn',
                'model_path': 'model.pkl',
                'python_version': '3.10.13',
                'env': {
                    'conda': 'conda.yaml',
                }
            }
        },
        'mlflow_version': '2.10.0',
        'model_size_bytes': os.path.getsize(os.path.join(model_dir, 'model.pkl')),
        'model_uuid': 'test-sklearn-001',
        'run_id': 'test-run-001',
        'utc_time_created': '2026-03-03 12:00:00.000000'
    }

    with open(os.path.join(model_dir, 'MLmodel'), 'w') as f:
        yaml.dump(mlmodel, f, default_flow_style=False)

    # Create conda.yaml
    conda = {
        'channels': ['defaults', 'conda-forge'],
        'dependencies': [
            'python=3.10.13',
            {'pip': [
                'scikit-learn==1.3.0',
                'mlserver==1.7.1',
                'mlserver-sklearn',
            ]}
        ]
    }

    with open(os.path.join(model_dir, 'conda.yaml'), 'w') as f:
        yaml.dump(conda, f, default_flow_style=False)

    print(f"Test model created in {model_dir}")
    print("Files:")
    for f in os.listdir(model_dir):
        print(f"  - {f}")

if __name__ == "__main__":
    create_test_model()
