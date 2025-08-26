pipeline {
    agent {
        kubernetes {
            label 'golang-agent'
            yaml """
apiVersion: v1
kind: Pod
metadata:
  labels:
    jenkins/label: golang-agent
spec:
  containers:
  - name: golang
    image: golang:1.23
    command:
    - cat
    tty: true
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  - name: docker
    image: docker:27.2.0-dind
    securityContext:
      privileged: true
    command:
    - dockerd-entrypoint.sh
    args:
    - --host=tcp://0.0.0.0:2375
    - --host=unix:///var/run/docker.sock
    env:
    - name: DOCKER_TLS_CERTDIR
      value: ""
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  - name: jnlp
    image: jenkins/inbound-agent:latest
    env:
    - name: JENKINS_AGENT_WORKDIR
      value: /home/jenkins/agent
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  volumes:
  - name: workspace-volume
    emptyDir: {}
"""
        }
    }

    environment {
        TARGETOS = 'linux'
        TARGETARCH = 'amd64'
    }

    stages {
        stage('Checkout SCM') {
            steps {
                checkout scm
                sh 'git config --global --add safe.directory $(pwd)'
            }
        }

        stage('Lint') {
            container('golang') {
                steps {
                    sh '''
                    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.60.3
                    ./bin/golangci-lint run --timeout=5m -v
                    '''
                }
            }
        }

        stage('Test') {
            container('golang') {
                steps {
                    sh 'go test ./...'
                }
            }
        }

        stage('Build Binary') {
            container('golang') {
                steps {
                    sh "make build TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH}"
                }
            }
        }

        stage('Docker Build & Push') {
            container('docker') {
                withCredentials([string(credentialsId: 'CR_PAT', variable: 'CR_PAT')]) {
                    steps {
                        sh '''
                        # Получаем краткий хэш текущего коммита
                        COMMIT_HASH=$(git rev-parse --short HEAD)
                        VERSION="v1.0.0-${COMMIT_HASH}"
                        IMAGE="ghcr.io/armrcode/kbot:${VERSION}-${TARGETOS}-${TARGETARCH}"

                        echo "Building image ${IMAGE}"

                        docker login ghcr.io -u jenkins --password-stdin <<< "$CR_PAT"
                        docker buildx create --use --name multiarch
                        docker buildx build --platform ${TARGETOS}/${TARGETARCH} -t ${IMAGE} -t ghcr.io/armrcode/kbot:latest --push .
                        '''
                    }
                }
            }
        }

        stage('Update Helm Chart') {
            container('docker') {
                withCredentials([string(credentialsId: 'CR_PAT', variable: 'CR_PAT')]) {
                    steps {
                        sh '''
                        COMMIT_HASH=$(git rev-parse --short HEAD)
                        VERSION="v1.0.0-${COMMIT_HASH}"
                        IMAGE="ghcr.io/armrcode/kbot:${VERSION}-${TARGETOS}-${TARGETARCH}"

                        echo "Updating helm/values.yaml with tag ${VERSION} and arch ${TARGETARCH}"

                        sed -i "s|^  tag:.*|  tag: ${VERSION}|" helm/values.yaml
                        sed -i "s|^  arch:.*|  arch: ${TARGETARCH}|" helm/values.yaml
                        sed -i "s|^  full:.*|  full: ${IMAGE}|" helm/values.yaml
                        '''
                    }
                }
            }
        }
    }

    post {
        always {
            echo 'Pipeline finished'
        }
    }
}
